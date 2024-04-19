package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"math"
	"strconv"
	"strings"
	"time"

	pb "github.com/imroc/keda-tencentcloud-clb-scaler/externalscaler"
	clb "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/clb/v20180317"
	"github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common"
	"github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common/profile"
	monitor "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/monitor/v20180724"
)

type ClbScaler struct {
	pb.ExternalScalerServer
	monitorClient          *monitor.Client
	clbClient              *clb.Client
	clbCache               *ClbCache
	publicClbMetricCache   *MetricCache
	internalClbMetricCache *MetricCache
}

func (scaler *ClbScaler) IsActive(ctx context.Context, _ *pb.ScaledObjectRef) (*pb.IsActiveResponse, error) {
	req := monitor.NewDescribeProductListRequest()
	req.Module = common.StringPtr("monitor")
	req.Limit = common.Uint64Ptr(1)
	_, err := scaler.monitorClient.DescribeProductListWithContext(ctx, req)
	if err != nil {
		return nil, err
	}
	result := &pb.IsActiveResponse{
		Result: true,
	}
	return result, nil
}

func (scaler *ClbScaler) StreamIsActive(_ *pb.ScaledObjectRef, sias pb.ExternalScaler_StreamIsActiveServer) error {
	return errors.New("external push is not supported")
}

func (scaler *ClbScaler) updateClbCache(ctx context.Context, loadBalancerId string) error {
	if clbInfo := scaler.clbCache.Get(loadBalancerId); clbInfo != nil { // return if already cached
		return nil
	}
	req := clb.NewDescribeLoadBalancersRequest()
	req.LoadBalancerIds = append(req.LoadBalancerIds, common.StringPtr(loadBalancerId))
	resp, err := scaler.clbClient.DescribeLoadBalancersWithContext(ctx, req)
	if err != nil {
		return err
	}
	set := resp.Response.LoadBalancerSet
	if l := len(set); l != 1 {
		switch l {
		case 0:
			return fmt.Errorf("%s is not found", loadBalancerId)
		default:
			return fmt.Errorf("found %d instances by loadBalancerId, response: %s", l, resp.ToJsonString())
		}
	}
	lb := set[0]
	clbInfo := &ClbInfo{
		ID:               *lb.LoadBalancerId,
		LoadBalancerType: LoadBalancerType(*lb.LoadBalancerType),
		VpcId:            *lb.VpcId,
	}
	if len(lb.LoadBalancerVips) > 0 {
		clbInfo.VIP = *lb.LoadBalancerVips[0]
	} else {
		if clbInfo.LoadBalancerType == LB_TYPE_INTERNAL {
			return fmt.Errorf("VIP not found for internal clb %s", loadBalancerId)
		}
	}
	scaler.clbCache.Set(loadBalancerId, clbInfo)
	return nil
}

func (scaler *ClbScaler) GetMetricSpec(ctx context.Context, obj *pb.ScaledObjectRef) (*pb.GetMetricSpecResponse, error) {
	metricName := obj.ScalerMetadata["metricName"]
	if metricName == "" {
		return nil, errors.New(`no "metricName" found in metadata`)
	}
	loadBalancerId := obj.ScalerMetadata["loadBalancerId"]
	if loadBalancerId == "" {
		return nil, errors.New(`no "loadBalancerId" found in metadata`)
	}
	err := scaler.updateClbCache(ctx, loadBalancerId)
	if err != nil {
		return nil, err
	}
	threshold := obj.ScalerMetadata["threshold"]
	if metricName == "" {
		return nil, errors.New(`no "threshold" found in metadata`)
	}
	intThreshold, err := strconv.ParseInt(threshold, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("threshold should be integer: %v", err)
	}
	resp := &pb.GetMetricSpecResponse{
		MetricSpecs: []*pb.MetricSpec{{
			MetricName: metricName,
			TargetSize: intThreshold,
		}},
	}
	return resp, nil
}

func (scaler *ClbScaler) newGetMonitorDataRequest(obj *pb.ScaledObjectRef) (req *monitor.GetMonitorDataRequest, clbInfo *ClbInfo, metricInfo *MetricInfo, err error) {
	lbId, ok := obj.ScalerMetadata["loadBalancerId"]
	if !ok {
		err = errors.New(`no "loadBalancerId" found in metadata`)
		return
	}
	clbInfo = scaler.clbCache.Get(lbId)
	if clbInfo == nil {
		err = fmt.Errorf("clb cache not found for %s", lbId)
		return
	}

	metricName, ok := obj.ScalerMetadata["metricName"]
	if !ok {
		err = errors.New(`no "metricName" found in metadata`)
		return
	}

	metricInfo, err = scaler.getMetricInfo(clbInfo.LoadBalancerType, metricName)
	if err != nil {
		return
	}
	req = monitor.NewGetMonitorDataRequest()
	req.Period = common.Uint64Ptr(metricInfo.MinPeriod)
	req.MetricName = common.StringPtr(metricName)

	var metricNamespace string
	var dimensions []*monitor.Dimension
	switch clbInfo.LoadBalancerType {
	case LB_TYPE_PUBLIC:
		metricNamespace = NS_PUBLIC_LB
		dimensions = append(dimensions, &monitor.Dimension{
			Name:  common.StringPtr("loadBalancerId"),
			Value: common.StringPtr(obj.ScalerMetadata["loadBalancerId"]),
		})
	case LB_TYPE_INTERNAL:
		metricNamespace = NS_INTERNAL_LB
		dimensions = append(dimensions, &monitor.Dimension{
			Name:  common.StringPtr("vip"),
			Value: common.StringPtr(clbInfo.VIP),
		})
		dimensions = append(dimensions, &monitor.Dimension{
			Name:  common.StringPtr("vpcId"),
			Value: common.StringPtr(clbInfo.VpcId),
		})
	default:
		err = fmt.Errorf("unknown LoadBalancerType %q", clbInfo.LoadBalancerType)
		return
	}
	req.Namespace = common.StringPtr(metricNamespace)

	listener, ok := obj.ScalerMetadata["listener"]
	if ok {
		ss := strings.Split(listener, "/")
		if len(ss) != 2 {
			err = fmt.Errorf(`bad listener format %q, correct format is "PROTOCOL/PORT"`, listener)
			return
		}
		dimensions = append(dimensions, &monitor.Dimension{
			Name:  common.StringPtr("protocol"),
			Value: common.StringPtr(ss[0]),
		})
		dimensions = append(dimensions, &monitor.Dimension{
			Name:  common.StringPtr("loadBalancerPort"),
			Value: common.StringPtr(ss[1]),
		})
	}
	req.Instances = append(req.Instances, &monitor.Instance{
		Dimensions: dimensions,
	})

	req.StartTime = common.StringPtr(time.Now().Add(-120 * time.Second).Format(time.RFC3339))
	req.SpecifyStatistics = common.Int64Ptr(metricInfo.SpecifyStatistics)
	return
}

func (scaler *ClbScaler) GetMetrics(ctx context.Context, gmr *pb.GetMetricsRequest) (*pb.GetMetricsResponse, error) {
	req, clbInfo, metricInfo, err := scaler.newGetMonitorDataRequest(gmr.ScaledObjectRef)
	if err != nil {
		return nil, err
	}
	resp, err := scaler.monitorClient.GetMonitorDataWithContext(ctx, req)
	if err != nil {
		return nil, err
	}
	if len(resp.Response.DataPoints) == 0 || len(resp.Response.DataPoints[0].MaxValues) == 0 {
		return nil, errors.New("no data points found")
	}
	dataPoint := resp.Response.DataPoints[0]
	metricValue := dataPoint.MaxValues[len(dataPoint.MaxValues)-1] // get the latest data point
	lis := gmr.ScaledObjectRef.ScalerMetadata["listener"]
	if lis != "" {
		lis = "listener " + lis + ","
	}
	log.Printf("metric %s(%s) of %s(%s%s) is %f(unit:%s)\n", metricInfo.MetricName, metricInfo.MetricEName, clbInfo.ID, lis, clbInfo.LoadBalancerType, *metricValue, metricInfo.Unit)
	return &pb.GetMetricsResponse{
		MetricValues: []*pb.MetricValue{
			{
				MetricName:  gmr.MetricName,
				MetricValue: int64(*metricValue),
			},
		},
	}, nil
}

func (scaler *ClbScaler) getMetricInfo(lbType LoadBalancerType, metricName string) (info *MetricInfo, err error) {
	switch lbType {
	case LB_TYPE_PUBLIC:
		info = scaler.publicClbMetricCache.Get(metricName)
		if info == nil {
			err = fmt.Errorf("no metric info found for %s with %s type", metricName, lbType)
			return
		}
		return
	case LB_TYPE_INTERNAL:
		info = scaler.internalClbMetricCache.Get(metricName)
		if info == nil {
			err = fmt.Errorf("no metric info found for %s with %s type", metricName, lbType)
			return
		}
		return
	}
	err = fmt.Errorf("unknown LoadBalancerType %s", lbType)
	return
}

func (scaler *ClbScaler) initMetricCache(ns string) (*MetricCache, error) {
	req := monitor.NewDescribeBaseMetricsRequest()
	req.Namespace = common.StringPtr(ns)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	resp, err := scaler.monitorClient.DescribeBaseMetricsWithContext(ctx, req)
	if err != nil {
		return nil, err
	}
	set := resp.Response.MetricSet
	if len(set) == 0 {
		return nil, fmt.Errorf("no metrics found for %s, response: %s", ns, resp.ToJsonString())
	}
	cache := newMetricCache()
	for _, metric := range set {
		var minPeriod uint64 = math.MaxUint64
		var slatType []*string
		for _, period := range metric.Periods {
			p, err := strconv.ParseUint(*period.Period, 10, 64)
			if err != nil {
				log.Println(err)
				continue
			}
			if p < minPeriod {
				minPeriod = p
				slatType = period.StatType
			}
		}
		haveMax := false
		for _, t := range slatType {
			if *t == "max" {
				haveMax = true
				break
			}
		}
		var specifyStatistics int64 = 2
		if !haveMax {
			switch *slatType[0] {
			case "avg":
				specifyStatistics = 1
			case "sum":
				specifyStatistics = 3
			case "min":
				specifyStatistics = 4
			}
		}
		info := &MetricInfo{
			MetricName:        *metric.MetricName,
			MetricEName:       *metric.MetricEName,
			Unit:              *metric.Unit,
			MinPeriod:         minPeriod,
			SpecifyStatistics: specifyStatistics,
		}
		cache.Set(*metric.MetricName, info)
	}
	return cache, nil
}

func newClbScaler(region, secretId, secretKey string) (*ClbScaler, error) {
	credential := common.NewCredential(secretId, secretKey)
	monitorClient, err := monitor.NewClient(credential, region, profile.NewClientProfile())
	if err != nil {
		return nil, err
	}
	clbClient, err := clb.NewClient(credential, region, profile.NewClientProfile())
	if err != nil {
		return nil, err
	}
	scaler := &ClbScaler{
		ExternalScalerServer: pb.UnimplementedExternalScalerServer{},
		monitorClient:        monitorClient,
		clbClient:            clbClient,
		clbCache:             newClbCache(),
	}
	scaler.publicClbMetricCache, err = scaler.initMetricCache(NS_PUBLIC_LB)
	if err != nil {
		return nil, err
	}
	scaler.internalClbMetricCache, err = scaler.initMetricCache(NS_INTERNAL_LB)
	if err != nil {
		return nil, err
	}
	return scaler, nil
}
