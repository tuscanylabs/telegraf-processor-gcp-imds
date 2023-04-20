package gcp

import (
	"context"
	_ "embed"
	"errors"
	"fmt"
	"time"

	"github.com/coocood/freecache"
	"github.com/influxdata/telegraf"
	"github.com/influxdata/telegraf/config"
	"github.com/influxdata/telegraf/plugins/common/parallel"
	"github.com/influxdata/telegraf/plugins/processors"
	"github.com/tuscanylabs/telegraf-processor-gcp-imds/internal/imds"
)

//go:embed sample.conf
var sampleConfig string

type GCPIMDSProcessor struct {
	ImdsTags         []string        `toml:"imds_tags"`
	Timeout          config.Duration `toml:"timeout"`
	CacheTTL         config.Duration `toml:"cache_ttl"`
	Ordered          bool            `toml:"ordered"`
	MaxParallelCalls int             `toml:"max_parallel_calls"`
	Log              telegraf.Logger `toml:"-"`
	TagCacheSize     int             `toml:"tag_cache_size"`
	LogCacheStats    bool            `toml:"log_cache_stats"`

	tagCache *freecache.Cache

	imdsClient          *imds.Client
	imdsTagsMap         map[string]struct{}
	parallel            parallel.Parallel
	cancelCleanupWorker context.CancelFunc
	workerContext       context.Context
}

const (
	DefaultMaxOrderedQueueSize = 10_000
	DefaultMaxParallelCalls    = 10
	DefaultTimeout             = 10 * time.Second
	DefaultCacheTTL            = 0 * time.Hour
	DefaultCacheSize           = 1000
	DefaultLogCacheStats       = false
)

var allowedImdsTags = map[string]struct{}{
	"hostname":    {},
	"machineType": {},
	"image":       {},
	"id":          {},
	"zone":        {},
}

func (*GCPIMDSProcessor) SampleConfig() string {
	return sampleConfig
}

func (r *GCPIMDSProcessor) Add(metric telegraf.Metric, _ telegraf.Accumulator) error {
	r.parallel.Enqueue(metric)
	return nil
}

func (r *GCPIMDSProcessor) logCacheStatistics() {
	if r.tagCache == nil {
		return
	}

	ticker := time.NewTicker(30 * time.Second)

	for {
		select {
		case <-r.workerContext.Done():
			return
		case <-ticker.C:
			r.Log.Debugf("cache: size=%d hit=%d miss=%d full=%d\n",
				r.tagCache.EntryCount(),
				r.tagCache.HitCount(),
				r.tagCache.MissCount(),
				r.tagCache.EvacuateCount(),
			)
			r.tagCache.ResetStatistics()
		}
	}
}

func (r *GCPIMDSProcessor) Init() error {
	r.Log.Debug("Initializing GCP IMDS Processor")
	if len(r.ImdsTags) == 0 {
		return errors.New("no tags specified in configuration")
	}

	for _, tag := range r.ImdsTags {
		if len(tag) == 0 || !isImdsTagAllowed(tag) {
			return fmt.Errorf("not allowed metadata tag specified in configuration: %s", tag)
		}
		r.imdsTagsMap[tag] = struct{}{}
	}
	if len(r.imdsTagsMap) == 0 {
		return errors.New("no allowed metadata tags specified in configuration")
	}

	return nil
}

func (r *GCPIMDSProcessor) Start(acc telegraf.Accumulator) error {
	r.tagCache = freecache.NewCache(r.TagCacheSize)
	if r.LogCacheStats {
		go r.logCacheStatistics()
	}

	r.Log.Debugf("cache: size=%d\n", r.TagCacheSize)
	if r.CacheTTL > 0 {
		r.Log.Debugf("cache timeout: seconds=%d\n", int(time.Duration(r.CacheTTL).Seconds()))
	}

	r.imdsClient = imds.NewClient()

	if r.Ordered {
		r.parallel = parallel.NewOrdered(acc, r.asyncAdd, DefaultMaxOrderedQueueSize, r.MaxParallelCalls)
	} else {
		r.parallel = parallel.NewUnordered(acc, r.asyncAdd, r.MaxParallelCalls)
	}

	return nil
}

func (r *GCPIMDSProcessor) Stop() {
	if r.parallel != nil {
		r.parallel.Stop()
	}
	r.cancelCleanupWorker()
}

func (r *GCPIMDSProcessor) LookupIMDSTags(metric telegraf.Metric) telegraf.Metric {
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(r.Timeout))
	defer cancel()

	var tagsNotFound []string

	for tag := range r.imdsTagsMap {
		val, err := r.tagCache.Get([]byte(tag))
		if err != nil {
			tagsNotFound = append(tagsNotFound, tag)
		} else {
			metric.AddTag(tag, string(val))
		}
	}

	if len(tagsNotFound) == 0 {
		return metric
	}

	iido, err := r.imdsClient.GetInstanceMetadata(
		ctx,
		&imds.GetInstanceMetadataInput{},
	)

	if err != nil {
		r.Log.Errorf("Error when calling GetInstanceMetadata: %v", err)
		return metric
	}

	for _, tag := range tagsNotFound {
		if v := getTagFromInstanceIdentityDocument(iido, tag); v != "" {
			metric.AddTag(tag, v)
			expiration := int(time.Duration(r.CacheTTL).Seconds())
			err = r.tagCache.Set([]byte(tag), []byte(v), expiration)
			if err != nil {
				r.Log.Errorf("Error when setting IMDS tag cache value: %v", err)
			}
		}
	}

	return metric
}

func (r *GCPIMDSProcessor) asyncAdd(metric telegraf.Metric) []telegraf.Metric {
	// Add IMDS Instance Identity Document tags.
	if len(r.imdsTagsMap) > 0 {
		metric = r.LookupIMDSTags(metric)
	}

	return []telegraf.Metric{metric}
}

func init() {
	processors.AddStreaming("gcp_imds", func() telegraf.StreamingProcessor {
		return newGCPIMDSProcessor()
	})
}

func newGCPIMDSProcessor() *GCPIMDSProcessor {
	ctx, cancel := context.WithCancel(context.Background())
	return &GCPIMDSProcessor{
		MaxParallelCalls:    DefaultMaxParallelCalls,
		TagCacheSize:        DefaultCacheSize,
		Timeout:             config.Duration(DefaultTimeout),
		CacheTTL:            config.Duration(DefaultCacheTTL),
		imdsTagsMap:         make(map[string]struct{}),
		workerContext:       ctx,
		cancelCleanupWorker: cancel,
	}
}

func getTagFromInstanceIdentityDocument(o *imds.GetMetadataInstanceOutput, tag string) string {
	switch tag {
	case "hostname":
		return o.Hostname
	case "image":
		return o.Image
	case "machineType":
		return o.MachineType
	case "id":
		return o.ID
	case "zone":
		return o.Zone
	default:
		return ""
	}
}

func isImdsTagAllowed(tag string) bool {
	_, ok := allowedImdsTags[tag]
	return ok
}
