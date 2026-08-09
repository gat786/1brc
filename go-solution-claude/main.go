// One Billion Row Challenge — annotated Go solution.
//
// The input file is one billion lines that look like this:
//
//     Hamburg;12.0
//     Bulawayo;8.9
//     Hamburg;-3.4
//
// For each station name we need the minimum, mean, and maximum temperature.
//
// THE CENTRAL IDEA
//
// One billion rows means every single thing you do "per row" happens a billion
// times. If one row costs you 100 nanoseconds, the program takes 100 seconds.
// So the whole game is: find work that happens per-row and delete it.
//
// The original version allocated 4 objects of memory per row (a string for the
// line, another for the trimmed line, a slice from Split, plus map churn).
// That's 4 BILLION allocations, and Go's garbage collector has to inspect all
// of them. This version allocates essentially nothing inside the loop.
//
// Read the file top to bottom: main() is at the bottom and describes the
// overall shape, so you may prefer to start there and jump back up.

package main

import (
	"bytes"
	"fmt"
	"math"
	"os"
	"runtime"
	"runtime/debug"
	"sort"
	"strings"
	"sync"
	"syscall"
)

// ---------------------------------------------------------------------------
// PART 1: How we store numbers
// ---------------------------------------------------------------------------
//
// Temperatures in this file always have exactly one decimal place and fit
// between -99.9 and 99.9. So instead of storing 12.3 as a float, we store the
// INTEGER 123 — the temperature measured in tenths of a degree. This is called
// "fixed-point" arithmetic.
//
// Two reasons:
//
//  1. CORRECTNESS. float32 can only hold about 7 significant digits. When you
//     add up 2.4 million temperatures the running total grows past 16,777,216,
//     and beyond that point float32 physically cannot represent the small
//     change from adding 20.5 — so the addition does nothing and your mean
//     comes out wrong. Integers have no such problem.
//
//  2. SPEED. Integer addition and comparison are faster than floating point,
//     and we skip parsing floats entirely.
//
// We only convert back to a decimal number at the very end, when printing.

// entry is the accumulated statistics for one weather station.
//
// Field types are chosen to keep the struct small, because a smaller struct
// means more entries fit in the CPU's cache, which means fewer slow trips to
// main memory:
//
//	min/max — int16 holds -32768..32767, and we only need -999..999.
//	sum     — int64, because summing a billion values needs the headroom.
//	count   — int32 holds up to ~2.1 billion, enough for 1 billion rows.
type entry struct {
	key      []byte // the station name (see the note in probe() about this)
	sum      int64  // total of all temperatures seen, in tenths
	count    int32  // how many readings we've seen
	min, max int16  // lowest and highest reading, in tenths
}

// ---------------------------------------------------------------------------
// PART 2: A hand-written hash table
// ---------------------------------------------------------------------------
//
// Go's built-in map is good, but it's built to handle any key type, resizing,
// deletion, and iteration safety. We need none of that. We know there are only
// a few hundred distinct station names and we never delete anything, so we can
// write something simpler and therefore faster.
//
// WHAT A HASH TABLE IS, BRIEFLY: we want to look up "Hamburg" without scanning
// a list of every station. So we run the name through a hash function, which
// turns text into a number that's spread out unpredictably ("Hamburg" -> some
// big number). We use that number to pick a slot in a big array. Next time we
// see "Hamburg" the hash function gives the same number, so we look in the same
// slot and find our data immediately.
//
// Sometimes two different names hash to the same slot. That's a "collision".
// We handle it with LINEAR PROBING: if the slot is taken by someone else, just
// try the next slot along, and keep walking until we find our name or an empty
// slot. With only a few hundred names in 131,072 slots, collisions are rare.

const (
	// The table has 131,072 slots for a few hundred stations. Wasteful on
	// purpose: an empty table means almost no collisions. The array is only a
	// few MB, which is nothing.
	//
	// It MUST be a power of two, because of the trick in tableMask below.
	tableSize = 1 << 17

	// To turn a huge hash number into a valid slot index we need it in the
	// range 0..131071. The obvious way is `hash % tableSize`, but division is
	// one of the slowest CPU instructions. Because tableSize is a power of two,
	// `hash & (tableSize-1)` gives the identical answer using a single bitwise
	// AND. tableMask is that tableSize-1 value: in binary it's seventeen 1
	// bits, so the AND simply keeps the low 17 bits and throws the rest away.
	tableMask = tableSize - 1
)

type table struct {
	slots []entry // the big mostly-empty array described above

	// A list of which slot indices we've actually filled in. Without this,
	// merging results at the end would mean scanning all 131,072 slots to find
	// the ~400 real ones. Cheap to maintain, saves pointless work later.
	used []int32
}

func newTable() *table {
	return &table{
		slots: make([]entry, tableSize),
		used:  make([]int32, 0, 1024), // room for 1024 stations before regrowing
	}
}

// probe finds the entry for a station name, creating it if this is the first
// time we've seen that name. It returns a POINTER, so the caller can update the
// statistics in place — no copying the struct in and back out again.
//
// The caller passes the hash in rather than having probe compute it, because
// the scanning loop already walks every byte of the name looking for the ';'
// separator, so it can build the hash along the way for almost free.
func (t *table) probe(key []byte, hash uint64) *entry {
	i := int32(hash & tableMask) // pick a starting slot (see tableMask above)
	for {
		e := &t.slots[i] // & means "pointer to", not a copy

		// An empty slot means we've never seen this name before. Claim it.
		if e.key == nil {
			e.key = key

			// Start min at the largest possible value and max at the smallest,
			// so that the very first real reading is guaranteed to replace
			// both. This avoids needing a special "is this the first reading?"
			// check on every single row.
			e.min, e.max = math.MaxInt16, math.MinInt16

			t.used = append(t.used, i) // remember that this slot is now live
			return e
		}

		// Slot is occupied. Is it occupied by US, or by a colliding name?
		// Compare lengths first: it's a single integer comparison, and it
		// rejects most mismatches without touching the string contents at all.
		if len(e.key) == len(key) && bytes.Equal(e.key, key) {
			return e
		}

		// Collision with a different name — walk to the next slot and retry.
		// The & tableMask makes the index wrap around to 0 at the end of the
		// array, so the table behaves like a circle and we never run off it.
		i = (i + 1) & tableMask
	}
}

// ---------------------------------------------------------------------------
// PART 3: The hot loop — parsing bytes
// ---------------------------------------------------------------------------

// scan reads through one slice of the file and returns a filled-in table.
//
// `data` is not a copy of the file — it's a window directly onto the file's
// bytes in memory (see the mmap call in main). We read straight out of it and
// never copy anything.
//
// PRECONDITION: data must begin at the start of a line and end just after a
// '\n' (or at the very end of the file). split() below guarantees this. Relying
// on that lets the loop skip a lot of "am I past the end?" checking.
func scan(data []byte) *table {
	t := newTable()

	// i is our read position. We advance it manually as we consume each field,
	// rather than using a line reader, because a line reader would have to
	// allocate a string for every line.
	for i := 0; i < len(data); {
		// --- Read the station name, hashing as we go -------------------------
		start := i

		// FNV-1a hash. These two magic constants are just the values the FNV
		// algorithm specifies; you don't need to understand why they work. What
		// matters is the shape: for each byte, mix it in with XOR (^) then
		// multiply by a big prime. The multiplication smears the bits around so
		// that similar names ("Hamburg", "Hamburh") land in very different
		// slots.
		var hash uint64 = 0xcbf29ce484222325
		for data[i] != ';' {
			hash = (hash ^ uint64(data[i])) * 0x100000001b3
			i++
		}

		// The name, as a window onto the file — NOT a copy. This is the key
		// allocation we eliminated. Note this is why the file must stay mapped
		// until we're completely done: these slices point into it.
		name := data[start:i]
		i++ // step over the ';'

		// --- Read the temperature -------------------------------------------
		//
		// The format is always one of: "9.9", "99.9", "-9.9", "-99.9".
		// So rather than call a general-purpose float parser, we read the two
		// or three digits by hand.
		//
		// The trick throughout: in ASCII, the character '7' has the numeric
		// value 55 and '0' has 48, so `'7' - '0'` == 7. Subtracting '0' from a
		// digit character converts it to the number it represents.

		neg := false
		if data[i] == '-' {
			neg = true
			i++
		}

		v := int16(data[i] - '0') // first digit
		i++

		// If the next character isn't the decimal point, then there was a
		// second whole-number digit, e.g. "23.4" rather than "3.4". Shift what
		// we have left one decimal place (×10) and add the new digit.
		if data[i] != '.' {
			v = v*10 + int16(data[i]-'0')
			i++
		}

		i++ // step over the '.'

		// The tenths digit. Shifting left again is exactly what turns the value
		// into tenths: "23.4" becomes 2 -> 23 -> 234.
		v = v*10 + int16(data[i]-'0')

		i += 2 // step over that last digit AND the '\n' ending the line

		if neg {
			v = -v
		}

		// --- Record it -------------------------------------------------------
		e := t.probe(name, hash)
		if v < e.min {
			e.min = v
		}
		if v > e.max {
			e.max = v
		}
		e.sum += int64(v)
		e.count++
	}
	return t
}

// ---------------------------------------------------------------------------
// PART 4: Splitting the work across CPU cores
// ---------------------------------------------------------------------------

// split divides the file into n pieces so that n CPU cores can work at once.
//
// The subtlety: if we cut the file at exactly len/n bytes, the cut will almost
// certainly land in the middle of a line like "Ham|burg;12.0", and both halves
// would be garbage. So after computing each rough boundary we walk FORWARD
// until we pass a newline, and cut there instead. Chunks end up slightly
// different sizes, which doesn't matter.
//
// Nothing is copied here. Each returned []byte is just another window onto the
// same underlying file bytes.
func split(data []byte, n int) [][]byte {
	out := make([][]byte, 0, n)

	size := len(data) / n
	if size == 0 { // file smaller than the core count; don't bother splitting
		return [][]byte{data}
	}

	start := 0
	for start < len(data) {
		end := start + size // the rough boundary

		if end >= len(data) {
			end = len(data) // last chunk: just run to the end of the file
		} else {
			for end < len(data) && data[end] != '\n' {
				end++ // scan forward to the end of this line
			}
			if end < len(data) {
				end++ // include the '\n' itself, so the chunk ends cleanly
			}
		}

		out = append(out, data[start:end])
		start = end // next chunk begins exactly where this one stopped
	}
	return out
}

// ---------------------------------------------------------------------------
// PART 5: Putting it together
// ---------------------------------------------------------------------------

func main() {
	// Our loop allocates nothing, so there is nothing for the garbage collector
	// to collect — but by default it still periodically stops to check. Turning
	// it off removes that pointless overhead. Only safe BECAUSE the loop
	// doesn't allocate; in a normal program this would leak memory until the
	// process ran out.
	debug.SetGCPercent(-1)

	f, err := os.Open(os.Getenv("MEASUREMENTS_FILE"))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer f.Close()

	st, err := f.Stat() // we need the file size for the mmap call
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	// MEMORY MAPPING. Normally reading a file means the kernel copies bytes
	// from its own cache into a buffer you own — and for a 13 GB file that copy
	// is a lot of wasted work. mmap instead asks the kernel: "make this file
	// appear as a region of my memory." We get back a []byte covering the whole
	// file, and the kernel loads pieces of it on demand as we touch them.
	//
	// No copy, no read loop, no buffer to size. And when several goroutines
	// read different parts at once, the kernel loads those parts in parallel.
	//
	// (This is a Unix-only call. Windows needs a different API.)
	data, err := syscall.Mmap(int(f.Fd()), 0, int(st.Size()),
		syscall.PROT_READ,  // we only ever read
		syscall.MAP_SHARED) // share the kernel's existing page cache
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	// Unmapping invalidates every `name` slice we created, so this must not
	// happen until after the merge and print below. defer runs at the very end
	// of main, so we're fine.
	defer syscall.Munmap(data)

	// scan() assumes every line ends with '\n'. Warn rather than crash if the
	// file is unusual.
	if len(data) > 0 && data[len(data)-1] != '\n' {
		fmt.Fprintln(os.Stderr, "warning: file does not end in a newline")
	}

	// --- Run one goroutine per CPU core ------------------------------------
	//
	// The important design choice: each goroutine gets its OWN table. If they
	// shared one, every update would need a lock, and the cores would spend all
	// their time waiting for each other instead of working. Independent tables
	// mean zero coordination during the expensive phase; we combine the results
	// afterwards, once, cheaply.
	parts := split(data, runtime.NumCPU())
	tables := make([]*table, len(parts))

	var wg sync.WaitGroup // a counter that lets us wait for all goroutines
	for i, p := range parts {
		wg.Add(1)
		go func(i int, p []byte) {
			defer wg.Done() // signal completion however we exit
			// Each goroutine writes only to tables[i], so no two goroutines
			// ever touch the same memory. That's what makes this lock-free.
			tables[i] = scan(p)
		}(i, p)
	}
	wg.Wait() // block until every chunk is finished

	// --- Combine the per-core tables into one ------------------------------
	//
	// Now that the billion-row work is done, a plain Go map is perfectly fine:
	// this loop runs a few hundred times per core, not a billion times.
	final := make(map[string]*entry, 1024)
	for _, t := range tables {
		for _, idx := range t.used { // only the slots we actually filled
			e := &t.slots[idx]

			g, ok := final[string(e.key)]
			if !ok {
				cp := *e // copy, so we don't hold a pointer into t.slots
				final[string(e.key)] = &cp
				continue
			}

			// Same station seen by another core: fold the two together.
			// min/max/sum/count all combine cleanly, which is precisely why
			// splitting the file was safe in the first place.
			if e.min < g.min {
				g.min = e.min
			}
			if e.max > g.max {
				g.max = e.max
			}
			g.sum += e.sum
			g.count += e.count
		}
	}

	// --- Print, sorted alphabetically --------------------------------------
	names := make([]string, 0, len(final))
	for k := range final {
		names = append(names, k)
	}
	sort.Strings(names) // Go map iteration order is deliberately random

	// Build the whole output in memory, then write it in one syscall. Hundreds
	// of small writes to stdout would each cross into the kernel; one big write
	// crosses once.
	var sb strings.Builder
	sb.Grow(len(names) * 40) // pre-size to avoid regrowing the buffer

	for _, n := range names {
		e := final[n]

		// The mean, still in tenths, is sum/count. Rounding to one decimal
		// place means rounding that to a whole number of tenths, then dividing
		// by 10.
		//
		// Floor(x + 0.5) is "round half up": 2.5 becomes 3. Go's math.Round and
		// the %.1f verb both round half AWAY FROM ZERO, so -2.5 would become -3
		// instead of -2. The reference implementation uses Java's Math.round,
		// which is half-up, so we match it explicitly here. Without this a
		// handful of stations differ in the last digit and the answer is
		// rejected.
		mean := math.Floor(float64(e.sum)/float64(e.count)+0.5) / 10

		// min and max are exact tenths, so dividing by 10 is safe with no
		// rounding concerns.
		fmt.Fprintf(&sb, "%s=%.1f/%.1f/%.1f\n",
			n, float64(e.min)/10, mean, float64(e.max)/10)
	}

	os.Stdout.WriteString(sb.String())
}
