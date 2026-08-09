vmname := "brcvm"
big_measurements   := "measurements.txt"
medium_measurements := "measurements_100000000.txt"
small_measurements  := "measurements_10000000.txt"


create_vm:
  #!/bin/bash
  PWD=$(pwd)
  limactl start --param brcdir=$PWD --name {{vmname}} 1-brc-vm.yaml -y

shell:
  limactl shell {{vmname}}

stop_vm:
  limactl stop {{vmname}} --force

remove_vm:
  limactl remove {{vmname}} --force

destroy_vm: stop_vm remove_vm

p_run_brc_s:
  #!/bin/bash
  MEASUREMENTS_FILE=$PWD/challenge/data/{{small_measurements}} uv run python-solution/main.py

p_run_brc_gpt_s:
  #!/bin/bash
  MEASUREMENTS_FILE=$PWD/challenge/data/{{small_measurements}} uv run python-solution/gpt-sol.py

g_run_brc_s:
  #!/bin/bash
  MEASUREMENTS_FILE=$PWD/challenge/data/{{small_measurements}} go run go-solution/main.go

g_run_brc_m:
  #!/bin/bash
  MEASUREMENTS_FILE=$PWD/challenge/data/{{medium_measurements}} go run go-solution/main.go

g_run_brc_b:
  #!/bin/bash
  MEASUREMENTS_FILE=$PWD/challenge/data/{{big_measurements}} go run go-solution/main.go

gc_run_brc_s:
  #!/bin/bash
  MEASUREMENTS_FILE=$PWD/challenge/data/{{small_measurements}} go run go-solution-claude/main.go

gc_run_brc_m:
  #!/bin/bash
  MEASUREMENTS_FILE=$PWD/challenge/data/{{medium_measurements}} go run go-solution-claude/main.go

gc_run_brc_b:
  #!/bin/bash
  MEASUREMENTS_FILE=$PWD/challenge/data/{{big_measurements}} go run go-solution-claude/main.go
