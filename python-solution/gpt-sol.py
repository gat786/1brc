import os
import sys


def parse_temperature(value: bytes) -> int:
  negative = value[0] == 45  # ord("-")

  if negative:
    value = value[1:]

  if value[-1] == 10:  # newline
      value = value[:-1]

  dot_position = value.index(b".")

  whole = int(value[:dot_position])
  decimal = value[dot_position + 1] - 48

  temperature = whole * 10 + decimal
  return -temperature if negative else temperature


def process(file_name: str) -> int:
  # city -> [minimum, maximum, total, count]
  city_data: dict[bytes, list[int]] = {}

  with open(file_name, "rb", buffering=1024 * 1024) as file:
    for line in file:
      city_name, temp_text = line.split(b";", 1)
      temperature = parse_temperature(temp_text)

      stats = city_data.get(city_name)

      if stats is None:
        city_data[city_name] = [
          temperature,
          temperature,
          temperature,
          1,
        ]
        continue

      if temperature < stats[0]:
        stats[0] = temperature

      if temperature > stats[1]:
        stats[1] = temperature

      stats[2] += temperature
      stats[3] += 1

  for city in sorted(city_data):
    minimum, maximum, total, count = city_data[city]
    mean = total / count

    print(f"{city.decode()}={minimum / 10:.1f}//{mean / 10:.1f}/{maximum / 10:.1f}")
  return 0


if __name__ == "__main__":
  measurement_file = os.getenv("MEASUREMENTS_FILE")

  if measurement_file is None:
    print("MEASUREMENTS_FILE is not set", file=sys.stderr)
    sys.exit(1)

  sys.exit(process(measurement_file))
