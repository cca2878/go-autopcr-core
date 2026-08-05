package fold

import (
	"github.com/cca2878/go-autopcr-core/internal/client/gamestate"
	"github.com/cca2878/go-autopcr-core/internal/client/internal/discovery"
	"github.com/cca2878/go-autopcr-core/internal/client/internal/protocol"
	"github.com/cca2878/go-autopcr-core/internal/client/internal/protocol/sdk"
)

// 免凭证发现握手的折叠器：资源版本与 CDN 根，供 masterdata 装配。
func maintenance(s *gamestate.PlayerState, _ protocol.Request, resp any) {
	m := resp.(*sdk.SourceIniGetMaintenanceStatusResponse)
	s.ResVer = m.ResVer
	s.ManifestVer = m.ManifestVer
	// res CDN 根的解析规则归 discovery（服务端发现域）所有，此处只做折叠。
	s.ResURLs = discovery.ResolveResURLs(m.ResHTTPType, m.Resource)
}
