#!/bin/bash
# scripts/generate-configs.sh

NODES=("node1" "node2" "node3")
for NODE in "${NODES[@]}"; do
  cp configs/agent-config-template.yaml configs/agent-config-${NODE}.yaml
  sed -i "s/REPLACE_HOST/${NODE}/g" configs/agent-config-${NODE}.yaml
done

echo "Generated configs for: ${NODES[*]}"
