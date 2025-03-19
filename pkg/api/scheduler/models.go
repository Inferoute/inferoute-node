package scheduler

import (
	"net/http"

	"github.com/robfig/cron/v3"
	"github.com/sentnl/inferoute-node/pkg/common"
)

// Service handles scheduling of periodic tasks
type Service struct {
	logger        *common.Logger
	cron          *cron.Cron
	baseURL       string
	internalKey   string
	httpClient    *http.Client
	isInitialized bool
}
