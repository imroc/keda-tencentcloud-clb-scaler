package main

import "sync"

type LoadBalancerType string

const (
	LB_TYPE_PUBLIC   LoadBalancerType = "OPEN"
	LB_TYPE_INTERNAL LoadBalancerType = "INTERNAL"
)

func (t LoadBalancerType) String() string {
	switch t {
	case LB_TYPE_PUBLIC:
		return "Public Type"
	case LB_TYPE_INTERNAL:
		return "Internal Type"
	default:
		return "Unknown Type"
	}
}

func (t LoadBalancerType) MetricNamespace() string {
	switch t {
	case LB_TYPE_PUBLIC:
		return NS_PUBLIC_LB
	case LB_TYPE_INTERNAL:
		return NS_INTERNAL_LB
	default:
		return ""
	}
}

type ClbInfo struct {
	ID               string
	LoadBalancerType LoadBalancerType
	VIP              string
	VpcId            string
}

type ClbCache struct {
	m *sync.Map
}

func newClbCache() *ClbCache {
	return &ClbCache{m: &sync.Map{}}
}

func (c *ClbCache) Set(lbId string, clb *ClbInfo) {
	c.m.Store(lbId, clb)
}

func (c *ClbCache) Get(lbId string) *ClbInfo {
	clb, ok := c.m.Load(lbId)
	if !ok {
		return nil
	}
	return clb.(*ClbInfo)
}
