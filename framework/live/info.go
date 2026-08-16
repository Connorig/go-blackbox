// Package live 提供 SRS 直播流媒体对接层:
//   - 回调接收(SRS → gbx):6 类 HTTP 回调,解析后转发给业务注入的裁决函数
//   - API 客户端(gbx → SRS):查流/踢流/踢客户端
//
// 模块只做"对接",不内置业务规则:鉴权裁决由业务注入(OnPublish 等)。
package live

// PublishInfo on_publish 回调(推流鉴权,最关键)。
// 推流 URL 的 query(如 ?key=xxx)在 param 字段(带 ? 前缀),不在 stream 字段。
type PublishInfo struct {
	Action   string `json:"action"`    // "on_publish"
	ClientID string `json:"client_id"` // 连接 ID(踢流用)
	IP       string `json:"ip"`
	Vhost    string `json:"vhost"`
	App      string `json:"app"`
	Stream   string `json:"stream"` // 纯流名,不含 query
	Param    string `json:"param"`  // "?key=xxx"(含 ? 前缀)
}

// PlayInfo on_play 回调(播放鉴权:会员/付费/封禁)。
type PlayInfo struct {
	Action   string `json:"action"`
	ClientID string `json:"client_id"`
	IP       string `json:"ip"`
	Vhost    string `json:"vhost"`
	App      string `json:"app"`
	Stream   string `json:"stream"`
	Param    string `json:"param"`
	PageURL  string `json:"pageUrl"`
}

// UnpublishInfo on_unpublish 回调(下播通知:结算/状态更新)。
type UnpublishInfo struct {
	Action   string `json:"action"`
	ClientID string `json:"client_id"`
	IP       string `json:"ip"`
	Vhost    string `json:"vhost"`
	App      string `json:"app"`
	Stream   string `json:"stream"`
	Param    string `json:"param"`
}

// DvrInfo on_dvr 回调(录制完成:回放入库)。
type DvrInfo struct {
	Action   string `json:"action"`
	ClientID string `json:"client_id"`
	IP       string `json:"ip"`
	Vhost    string `json:"vhost"`
	App      string `json:"app"`
	Stream   string `json:"stream"`
	Param    string `json:"param"`
	Cwd      string `json:"cwd"`  // 录制文件目录
	File     string `json:"file"` // 录制文件相对路径
}

// ConnectInfo on_connect 回调(连接建立,可选,默认放行)。
type ConnectInfo struct {
	Action   string `json:"action"`
	ClientID string `json:"client_id"`
	IP       string `json:"ip"`
	Vhost    string `json:"vhost"`
	App      string `json:"app"`
	Stream   string `json:"stream"`
	Param    string `json:"param"`
	TCURL    string `json:"tcUrl"`
	PageURL  string `json:"pageUrl"`
}

// HlsInfo on_hls 回调(切片生成,后续回放用)。
type HlsInfo struct {
	Action   string `json:"action"`
	ClientID string `json:"client_id"`
	IP       string `json:"ip"`
	Vhost    string `json:"vhost"`
	App      string `json:"app"`
	Stream   string `json:"stream"`
	Param    string `json:"param"`
	Duration float64 `json:"duration"` // 切片时长(秒)
	CWD      string  `json:"cwd"`      // m3u8 目录
	File     string  `json:"file"`     // 切片文件
	URL      string  `json:"url"`      // m3u8 URL
	M3U8     string  `json:"m3u8"`
	SeqNo    int     `json:"seq_no"`
}
