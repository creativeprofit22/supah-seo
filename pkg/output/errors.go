package output

// Machine-readable error codes for the output envelope.
const (
	ErrInvalidURL        = "INVALID_URL"
	ErrConfigLoadFailed  = "CONFIG_LOAD_FAILED"
	ErrConfigSaveFailed  = "CONFIG_SAVE_FAILED"
	ErrConfigGetFailed   = "CONFIG_GET_FAILED"
	ErrProviderNotFound  = "PROVIDER_NOT_FOUND"
	ErrCrawlFailed       = "CRAWL_FAILED"
	ErrAuditFailed       = "AUDIT_FAILED"
	ErrReportWriteFailed = "REPORT_WRITE_FAILED"
	ErrReportListFailed  = "REPORT_LIST_FAILED"
	ErrFetchTimeout      = "FETCH_TIMEOUT"
	ErrCancelled         = "CANCELLED"
	ErrAuthRequired      = "AUTH_REQUIRED"
	ErrAuthFailed        = "AUTH_FAILED"
	ErrApprovalRequired  = "APPROVAL_REQUIRED"
	ErrEstimateFailed    = "ESTIMATE_FAILED"
	ErrSERPFailed        = "SERP_FAILED"
	ErrGSCFailed         = "GSC_FAILED"
	ErrAEOFailed         = "AEO_FAILED"
	ErrGEOFailed         = "GEO_FAILED"
	ErrLabsFailed        = "LABS_FAILED"
	ErrPSIFailed         = "PSI_FAILED"
	ErrBacklinksFailed   = "BACKLINKS_FAILED"
)
