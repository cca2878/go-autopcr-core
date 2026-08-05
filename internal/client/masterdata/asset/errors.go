package asset

import (
	"fmt"
	"net/http"

	"github.com/cca2878/go-autopcr-core/internal/errs"
)

var (
	// ErrNotInManifest 表示整棵清单树都展开完了，却没有目标资源的条目——要的东西这个版本
	// 里就是没有，换台主机再问还是没有。
	ErrNotInManifest = errs.DomainMasterdata.New(errs.KindRejected, "清单中未找到该资源")

	// ErrBadManifest 表示清单条目本身不可用（如缺少寻址所需的 md5 键）。
	ErrBadManifest = errs.DomainMasterdata.New(errs.KindCorrupt, "清单内容不可用")
)

// HTTPError 表示 CDN 对某个 URL 返回了非 200。
//
// 带上状态码，是为了让调用方分得清两件处置方式相反的事："这个资源/这个版本本来就不在"
// （4xx，换台主机再要一次还是 404）与"这台 CDN 这会儿不行"（5xx/429，换一台或稍后重试
// 就好）。见 Retryable。
type HTTPError struct {
	URL    string
	Status int
}

func (e *HTTPError) Error() string {
	return fmt.Sprintf("GET %s: 状态 %d", e.URL, e.Status)
}

// Retryable 报告这次失败是否值得换一台主机或稍后再试：5xx 是服务端侧故障，429 是限流，
// 二者都不代表资源不存在；4xx 的其余码则是"要的东西不对"，重试多少次都一样。
func (e *HTTPError) Retryable() bool {
	return e.Status >= http.StatusInternalServerError || e.Status == http.StatusTooManyRequests
}

// ErrorClass 按状态码定处置类别——同一个类型落在哪一类由字段决定，正是归类与具体错误类型
// 各司其职之处：503 值得换台主机重来（Transient），404 换到哪台都一样（Rejected）。
func (e *HTTPError) ErrorClass() errs.Class {
	if e.Retryable() {
		return errs.DomainMasterdata.With(errs.KindTransient)
	}
	return errs.DomainMasterdata.With(errs.KindRejected)
}
