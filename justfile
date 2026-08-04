vmname := "brcvm"

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

run_brc:
  #!/bin/bash
  MEASUREMENTS_FILE=$PWD/challenge/data/measurements.txt uv run python-solution/main.py

run_brc_gpt:
  #!/bin/bash
  MEASUREMENTS_FILE=$PWD/challenge/data/measurements.txt uv run python-solution/gpt-sol.py
