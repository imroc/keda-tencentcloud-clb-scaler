package main

import "sync"

type MetricInfo struct {
	MetricName        string
	MetricEName       string
	Unit              string
	MinPeriod         uint64
	SpecifyStatistics int64
}

type MetricCache struct {
	m *sync.Map
}

func newMetricCache() *MetricCache {
	return &MetricCache{
		m: &sync.Map{},
	}
}

func (c *MetricCache) Set(metricName string, info *MetricInfo) {
	c.m.Store(metricName, info)
}

func (c *MetricCache) Get(metricName string) *MetricInfo {
	v, ok := c.m.Load(metricName)
	if !ok {
		return nil
	}
	return v.(*MetricInfo)
}
