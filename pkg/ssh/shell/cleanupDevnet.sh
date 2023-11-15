#!/ust/bin/env bash
#name:TASK [stop node]
sudo systemctl stop avalanchego
#name:TASK [remove previous avalanchego db and logs]
rm -rf /home/ubuntu/.avalanchego/db/
rm -rf /home/ubuntu/.avalanchego/logs/
#name:TASK [start node]
shell: sudo systemctl start avalanchego
