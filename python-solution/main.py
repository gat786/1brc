from concurrent.futures import ThreadPoolExecutor
import os
import sys
import math
import tempfile

from typing_extensions import Dict, Tuple


def process(file_name: str):
  # Tuple[int, int, int, int]- min, max, total, count
  city_data: Dict[str,Tuple[float, float, float, int]] = {}
  with open(file=file_name, mode="r") as mfp:
    for line in mfp:
      city_name, temp = line.split(";")
      temp_double = float(temp)
      if city_name in city_data:
        curr_min, curr_max, curr_total, curr_count = city_data[city_name]
        min_v, max_v, total_v, count_v = (
          min(curr_min, temp_double),
          max(curr_max, temp_double),
          float(format(curr_total + temp_double, '.2f')),
          curr_count + 1
        )
        city_data[city_name] = (min_v,max_v, total_v, count_v)
      else:
        city_data[city_name] = (temp_double, temp_double, temp_double, 1)

  for city, city_values in city_data.items():
    minv = city_values[0]
    maxv = city_values[1]
    meanv = city_values[2] / city_values[3]
    print(f"{city}={minv}/{meanv}/{maxv}")
  return 0

if __name__ == "__main__":
  mfile = os.getenv("MEASUREMENTS_FILE")
  if mfile == None:
    sys.exit(-1)
  total_cpus = os.cpu_count()
  if total_cpus == None:
    sys.exit(-1)

  t_size = os.path.getsize(mfile)
  chunk = math.floor(t_size / total_cpus)
  remaining_bytes = t_size % chunk
  all_chunks = [ i * chunk for i in range(total_cpus)]
  all_chunks[len(all_chunks) - 1] = all_chunks[len(all_chunks) - 1] + remaining_bytes

  process(file_name=mfile)
