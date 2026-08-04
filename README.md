# 1 Billion rows challenge

## Initial Plan

1. Used uv to initialise a python environment
2. Ran the `challenge/src/main/python/create_measurements.py` file to generate data.
3. create a template.yaml file using limactl and setup cpu and memory to be limited for this vm, along with mounting current dir to home
4. Installing maven in the vm using provision scripts.
5. Using zig to write code in the vm and test my output.

## Update

I dont know zig, I was excited to do and I did choose it.
I wrote a solution in python that took me more than a couple of hours. I then even used
chatgpt to improve its performance it still took more than 7 minutes to solve it.
It is still a single process application, so I believe if I were to split it into
multiple processes I will see performance improvements, although I do not know
how I can reach the 1 secs completion times that I see on the boards.
