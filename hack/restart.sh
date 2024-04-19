#!/bin/bash

set -x

kubectl --context=tke -n keda rollout restart deploy/tencentcloud-clb-scaler
kubectl --context=tke -n keda rollout status deploy/tencentcloud-clb-scaler --timeout=90s
sleep 3
newpod=$(kubectl get pod -l app=tencentcloud-clb-scaler -o jsonpath='{.items[?(@.status.phase=="Running")].metadata.name}')
if [-z $newpod]; then
	"no running pod"
	exit 1
fi
echo "new pod is $newpod"
kubectl --context=tke -n keda wait --for='jsonpath={.status.conditions[?(@.type=="Ready")].status}=True' pod/${newpod}
