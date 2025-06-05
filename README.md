# KEDA External Scaler for TencentCloud CLB

keda-tencentcloud-clb-scaler 是基于腾讯云 CLB 监控指标的 KEDA External Scaler，可实现基于 CLB 连接数、QPS 和带宽等指标的弹性伸缩。

## 准备访问密钥

需要准备一个腾讯云账号的访问密钥(SecretID、SecretKey)，参考[子账号访问密钥管理](https://cloud.tencent.com/document/product/598/37140)，要求账号至少具有以下权限：

```json
{
    "version": "2.0",
    "statement": [
        {
            "effect": "allow",
            "action": [
                "clb.DescribeLoadBalancers",
                "monitor.DescribeProductList",
                "monitor.GetMonitorData",
                "monitor.DescribeBaseMetrics"
            ],
            "resource": [
                "*"
            ]
        }
    ]
}
```

## 安装

```yaml
helm repo add clb-scaler https://imroc.github.io/keda-tencentcloud-clb-scaler
helm upgrade --install clb-scaler clb-scaler/clb-scaler -n keda \
  --set region="ap-chengdu" \
  --set credentials.secretId="xxx" \
  --set credentials.secretKey="xxx"
```

* `region` 修改为CLB 所在地域（一般就是集群所在地域），地域列表: https://cloud.tencent.com/document/product/213/6091
* `credentials.secretId` 和 `credentials.secretKey`  是腾讯云账户密钥对，用于查 CLB 监控数据。

## ScaledObject 配置方法

基于 CLB 的监控指标通常用于在线业务，通常使用 KEDA 的 `ScaledObject` 配置弹性伸缩，配置 `external` 类型的 trigger，并传入所需的 metadata，主要包含以下字段：
* `scalerAddress` 是 `keda-operator` 调用 `keda-tencentcloud-clb-scaler` 时使用的地址。
* `loadBalancerId` 是 CLB 的实例 ID。
* `metricName` 是 CLB 的监控指标名称，公网和内网的大部分指标相同，具体指标列表参考官方文档 [公网负载均衡监控指标](https://cloud.tencent.com/document/product/248/51898) 和 [内网负载均衡监控指标](https://cloud.tencent.com/document/product/248/51899)。
* `threshold` 是扩缩容的指标阈值，即会通过比较 `metricValue / Pod 数量` 与 `threshold` 的值来决定是否扩缩容。
* `listener` 是唯一可选的配置，指定监控指标的 CLB 监听器，格式：`协议/端口`。

## 配置示例

下面给出一些常用的配置示例。

### 基于 CLB 连接数指标的弹性伸缩

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
    - type: external
      metadata:
        scalerAddress: clb-scaler.keda.svc.cluster.local:9000
        loadBalancerId: lb-xxxxxxxx
        metricName: ClientConnum # 连接数指标
        threshold: "100" # 每个 Pod 处理 100 条连接
        listener: "TCP/9090" # 可选，指定监听器，格式：协议/端口
```

### 基于 CLB QPS 指标的弹性伸缩

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
    - type: external
      metadata:
        scalerAddress: clb-scaler.keda.svc.cluster.local:9000
        loadBalancerId: lb-xxxxxxxx
        metricName: TotalReq # 每秒连接数指标
        threshold: "500" # 平均每个 Pod 支撑 500 QPS
        listener: "TCP/443" # 可选，指定监听器，格式：协议/端口
```
