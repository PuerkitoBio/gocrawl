package gocrawl

// Enum indicating the reason why the crawling ended.
type EndReason uint8

const (
	ErDone EndReason = iota
	ErMaxVisits
	ErError
)

// Enum indicating the kind of the crawling error.
type CrawlErrorKind uint8

const (
	CekFetch CrawlErrorKind = iota
	CekParseRobots
	CekHttpStatusCode
	CekReadBody
	CekParseBody
	CekParseSeed
	CekParseNormalizedSeed
	CekProcessLinks
	CekParseRedirectUrl
)

// Enum indicating the head request override mode (the default mode is specified
// in the Options of the crawler).
type HeadRequestMode uint8

const (
	HrmDefault HeadRequestMode = iota
	HrmRequest
	HrmIgnore
)

// Enum indicating to the crawler and the Filter extender function
// the origin of the URL to crawl.
type EnqueueOrigin int

const (
	EoSeed        EnqueueOrigin = iota // Seed URLs have this source
	EoHarvest                          // URLs harvested from a visit to a page have this source
	EoRedirect                         // URLs enqueued from a fetch redirection have this source by default
	EoError                            // URLs enqueued after an error
	EoCustomStart                      // Custom EnqueueOrigins should start at this value instead of iota
)
