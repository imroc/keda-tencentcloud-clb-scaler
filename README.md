# KEDA External Scaler for TencentCloud CLB

作为 KEDA 的 External Scaler，基于腾讯云 CLB 的监控数据实现自动伸缩（如 CLB 的连接数、流量、QPS 等数据）。

## 用法

基于 CLB 连接数伸缩：

```yaml
apiVersion: keda.sh/v1alpha1
kind: ScaledObject
metadata:
  name: httpbin
spec:
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: httpbin
  pollingInterval: 15
  minReplicaCount: 1
  maxReplicaCount: 100
  triggers:
    - type: external-push
      metadata:
        scalerAddress: tencentcloud-clb-scaler:9000
        region: ap-guangzhou
        loadBalancerId: lb-xxxxxxxx
        metricName: ClientConnum
        threshold: "100" # 每个 Pod 处理 100 条连接
        listener: "TCP/9090" # 可选，指定监听器，格式：协议/端口
```

基于 CLB HTTP 监听器的 QPS 伸缩：

```yaml
apiVersion: keda.sh/v1alpha1
kind: ScaledObject
metadata:
  name: httpbin
spec:
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: httpbin
  pollingInterval: 15
  minReplicaCount: 1
  maxReplicaCount: 100
  triggers:
    - type: external-push
      metadata:
        scalerAddress: tencentcloud-clb-scaler:9000
        region: ap-chengdu
        loadBalancerId: lb-xxxxxxxx
        metricName: TotalReq
        threshold: "500" # 平均每个 Pod 支撑 500 QPS
        listener: "TCP/443" # 可选，指定监听器，格式：协议/端口
```


## 参考资料

* [公网负载均衡监控指标](https://cloud.tencent.com/document/product/248/51898)
* [内网负载均衡监控指标](https://cloud.tencent.com/document/product/248/51899)

