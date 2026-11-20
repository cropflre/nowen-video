// Package mcp 给 nowen-video 提供 MCP server（第一阶段：仅 stdio 传输）。
//
// 外部 AI 客户端通过 `server-lite mcp` 子命令以 JSON-RPC 2.0 / stdio 接入，
// 对影片做查询、分级建议/确认、刮削与元数据增删改查。
// 工具只包确定性的非 LLM 能力，不调用内置 AI。
// stdio 模式没有 token：进程边界即信任边界，只允许本机可信客户端启动；
// 第一阶段不开放远程 HTTP 传输。
package mcp

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Server 包装官方 MCP SDK 的 server。
type Server struct {
	svc *Service
	srv *mcp.Server
}

// NewServer 创建 MCP server 并注册第一阶段的全部工具。
func NewServer(svc *Service, version string) *Server {
	srv := mcp.NewServer(&mcp.Implementation{Name: "nowen-video", Version: version}, nil)
	s := &Server{svc: svc, srv: srv}
	s.registerTools()
	return s
}

// Run 在 stdio 传输上运行 server，直到 stdin 关闭或 ctx 取消。
func (s *Server) Run(ctx context.Context) error {
	return s.srv.Run(ctx, &mcp.StdioTransport{})
}

// ==================== 工具入参 ====================

type searchMediaInput struct {
	Query        string `json:"query,omitempty" jsonschema:"关键词，按标题、原始标题、类型模糊匹配；留空列出全部"`
	Genre        string `json:"genre,omitempty" jsonschema:"类型过滤，匹配 genres 字段；留空不过滤"`
	RatingStatus string `json:"rating_status,omitempty" jsonschema:"分级确认状态：confirmed 已确认，unconfirmed 未确认，all 全部；默认 all"`
	Kind         string `json:"kind,omitempty" jsonschema:"movie 影片，series 剧集，all 全部；默认 all"`
	Page         int    `json:"page,omitempty" jsonschema:"页码，从 1 开始；默认 1"`
	PageSize     int    `json:"page_size,omitempty" jsonschema:"每页条数；默认 20，最大 100"`
}

type targetInput struct {
	Kind string `json:"kind,omitempty" jsonschema:"movie 影片，series 剧集；默认 movie"`
	ID   string `json:"id" jsonschema:"影片或剧集的 id"`
}

type listUnconfirmedInput struct {
	Kind     string `json:"kind,omitempty" jsonschema:"movie 影片，series 剧集，all 全部；默认 all"`
	Page     int    `json:"page,omitempty" jsonschema:"页码，从 1 开始；默认 1"`
	PageSize int    `json:"page_size,omitempty" jsonschema:"每页条数；默认 20，最大 100"`
}

type setRatingInput struct {
	Kind  string `json:"kind,omitempty" jsonschema:"movie 影片，series 剧集；默认 movie。剧集作用于当前全部剧集"`
	ID    string `json:"id" jsonschema:"影片或剧集的 id"`
	Level string `json:"level" jsonschema:"家长确认分级：G、PG、PG-13、R、NC-17；空字符串表示撤回确认"`
}

type updateMetadataInput struct {
	Kind      string  `json:"kind,omitempty" jsonschema:"movie 影片，series 剧集；默认 movie"`
	ID        string  `json:"id" jsonschema:"影片或剧集的 id"`
	Title     *string `json:"title,omitempty" jsonschema:"标题；不提供则不改"`
	OrigTitle *string `json:"orig_title,omitempty" jsonschema:"原始标题；不提供则不改"`
	Year      *int    `json:"year,omitempty" jsonschema:"年份，1888-2100；不提供则不改"`
	Overview  *string `json:"overview,omitempty" jsonschema:"简介；不提供则不改"`
	Genres    *string `json:"genres,omitempty" jsonschema:"类型，逗号分隔；不提供则不改"`
}

func (s *Server) registerTools() {
	mcp.AddTool(s.srv, &mcp.Tool{
		Name:        "search_media",
		Description: "搜索影片或剧集：按关键词、类型、分级确认状态过滤，分页返回。结果含当前家庭分级、确认状态与分级建议。",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in searchMediaInput) (*mcp.CallToolResult, any, error) {
		out, err := s.svc.SearchMedia(in.Query, in.Genre, in.RatingStatus, in.Kind, in.Page, in.PageSize)
		if err != nil {
			return nil, nil, err
		}
		return nil, out, nil
	})

	mcp.AddTool(s.srv, &mcp.Tool{
		Name:        "get_media",
		Description: "取一部影片或剧集的详情：标题、年份、类型、时长、评分、MPAA、TMDb 原始分级数据、家庭分级与确认状态、分级建议。",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in targetInput) (*mcp.CallToolResult, any, error) {
		out, err := s.svc.GetMedia(in.Kind, in.ID)
		if err != nil {
			return nil, nil, err
		}
		return nil, out, nil
	})

	mcp.AddTool(s.srv, &mcp.Tool{
		Name:        "list_unconfirmed_ratings",
		Description: "列出未确认分级的影片或剧集清单，分页返回，每项含分级建议。用于批量处理待确认内容。",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in listUnconfirmedInput) (*mcp.CallToolResult, any, error) {
		out, err := s.svc.ListUnconfirmedRatings(in.Kind, in.Page, in.PageSize)
		if err != nil {
			return nil, nil, err
		}
		return nil, out, nil
	})

	mcp.AddTool(s.srv, &mcp.Tool{
		Name:        "suggest_family_rating",
		Description: "只给分级建议，不写库。仅美国明确分级才给建议（冲突、混合、未知一律不建议）；建议不自动保存、不放行儿童，家长确认后才生效。",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in targetInput) (*mcp.CallToolResult, any, error) {
		out, err := s.svc.SuggestFamilyRating(in.Kind, in.ID)
		if err != nil {
			return nil, nil, err
		}
		return nil, out, nil
	})

	mcp.AddTool(s.srv, &mcp.Tool{
		Name:        "set_family_rating",
		Description: "写家长确认分级（G、PG、PG-13、R、NC-17；空字符串撤回确认）。默认关闭，需 mcp.allow_direct_rating=true 才允许；每次写入都记审计。剧集作用于当前全部剧集，新入库的剧集仍需再次确认。",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in setRatingInput) (*mcp.CallToolResult, any, error) {
		out, err := s.svc.SetFamilyRating(in.Kind, in.ID, in.Level)
		if err != nil {
			return nil, nil, err
		}
		return nil, out, nil
	})

	mcp.AddTool(s.srv, &mcp.Tool{
		Name:        "update_media_metadata",
		Description: "改元数据，只接受白名单字段：title、orig_title、year、overview、genres；其它字段会被拒绝。每次写入都记审计（改前改后）。",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in updateMetadataInput) (*mcp.CallToolResult, any, error) {
		out, err := s.svc.UpdateMediaMetadata(in.Kind, in.ID, MetadataUpdateInput{
			Title: in.Title, OrigTitle: in.OrigTitle, Year: in.Year,
			Overview: in.Overview, Genres: in.Genres,
		})
		if err != nil {
			return nil, nil, err
		}
		return nil, out, nil
	})

	mcp.AddTool(s.srv, &mcp.Tool{
		Name:        "rescrape_media",
		Description: "对一部影片或剧集触发一次 TMDb 重刮，复用服务端现有刮削逻辑（不调内置 AI）。失败时返回错误，不改库。",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in targetInput) (*mcp.CallToolResult, any, error) {
		out, err := s.svc.RescrapeMedia(in.Kind, in.ID)
		if err != nil {
			return nil, nil, err
		}
		return nil, out, nil
	})

	mcp.AddTool(s.srv, &mcp.Tool{
		Name:        "clear_tmdb_rating_data",
		Description: "删除某部影片或剧集的刮削分级数据（只清 tmdb_rating_data 字段）。每次写入都记审计。",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in targetInput) (*mcp.CallToolResult, any, error) {
		out, err := s.svc.ClearTMDbRatingData(in.Kind, in.ID)
		if err != nil {
			return nil, nil, err
		}
		return nil, out, nil
	})

	mcp.AddTool(s.srv, &mcp.Tool{
		Name:        "clear_family_rating",
		Description: "把确认分级清回未确认（撤回家长确认）。默认关闭，需 mcp.allow_direct_rating=true 才允许；每次写入都记审计。",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in targetInput) (*mcp.CallToolResult, any, error) {
		out, err := s.svc.ClearFamilyRating(in.Kind, in.ID)
		if err != nil {
			return nil, nil, err
		}
		return nil, out, nil
	})
}
