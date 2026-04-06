import React, { useEffect, useState } from 'react';
import { 
    PlayWithExternalPlayer, OpenMediaFolder, ToggleFavorite, 
    ToggleWatched, GetMediaDetail, DeleteMedia, OpenNFO, 
    GetMediaFiles, GetMediaPreviews, PlayFile 
} from "../../wailsjs/go/main/App";
import { 
    ArrowLeft, Play, FolderOpen, Star, Eye, EyeOff, 
    Trash2, FileEdit, ChevronDown, Check
} from 'lucide-react';

interface MediaDetailProps {
    media: any;
    onClose: () => void;
    onSelectFilter: (filter: { type: string; value: string; label: string }) => void;
}

const MediaDetail: React.FC<MediaDetailProps> = ({ media, onClose, onSelectFilter }) => {
    const [detail, setDetail] = useState(media);
    const [msg, setMsg] = useState('');
    const [files, setFiles] = useState<string[]>([]);
    const [previews, setPreviews] = useState<string[]>([]);
    const [currFilePath, setCurrFilePath] = useState(media.file_path);
    const [showFileMenu, setShowFileMenu] = useState(false);

    useEffect(() => {
        const load = async () => {
            try {
                const [d, f, p] = await Promise.all([
                    GetMediaDetail(media.id),
                    GetMediaFiles(media.id),
                    GetMediaPreviews(media.id)
                ]);
                setDetail(d);
                setFiles(f);
                setPreviews(p);
                setCurrFilePath(d.file_path);
            } catch (e) { console.error(e); }
        };
        load();
    }, [media.id]);

    const handlePlay = () => PlayFile(currFilePath);
    const handleOpenDir = () => OpenMediaFolder(detail.id);
    const handleOpenNFO = async () => {
        try {
            await OpenNFO(detail.id);
            showMsg("已在系统编辑器中打开 .nfo 文件");
        } catch (e: any) { showMsg(e.toString()); }
    };
    const handleDelete = async () => {
        if (window.confirm("确定要从数据库中移除此条目吗？（注：不会删除本地文件）")) {
            try {
                await DeleteMedia(detail.id);
                onClose();
            } catch (e) { console.error(e); }
        }
    };
    
    const showMsg = (m: string) => { setMsg(m); setTimeout(()=>setMsg(''), 4000); };

    const handleFav = async () => {
        try {
            await ToggleFavorite(detail.id);
            setDetail({ ...detail, is_favorite: !detail.is_favorite });
            showMsg("收藏状态已更新");
        } catch (e) { console.error(e); }
    };
    
    const handleWatched = async () => {
        try {
            await ToggleWatched(detail.id);
            setDetail({ ...detail, is_watched: !detail.is_watched });
            showMsg("播放状态已更新");
        } catch (e) { console.error(e); }
    };

    // 智能提取编码/番号 (精修版)
    const getCode = () => {
        // 1. 优先取 NFO 中的 num
        if (detail.nfo_extra_fields) {
            try {
                const extra = JSON.parse(detail.nfo_extra_fields);
                if (extra.num) return extra.num.toString().toUpperCase();
            } catch(e){}
        }
        // 2. 尝试从文件名提取 (格式如 ABC-123 或 ABC-001 或 PRE-123)
        const filename = currFilePath?.split(/[\\/]/).pop() || "";
        // 更加宽泛的正则，匹配类似 XXX-123 的特征
        const codeMatch = filename.match(/([A-Z0-9]{2,10}-\d{2,6})/i);
        if (codeMatch) return codeMatch[1].toUpperCase();

        // 3. 兜底取 ID 前 8 位 (除非明确有 code 字段)
        return detail.code || detail.id?.slice(0, 8);
    };

    // 标签排序与清洗 (精修版)
    const getSortedTags = () => {
        const rawTags = detail.genres ? detail.genres.split(/[,，/ ]/).filter(Boolean) : [];
        const resolutionList = ["4K", "1080P", "720P", "UHD", "HD", "FHD", "SD"];
        const codecList = ["H265", "HEVC", "H264", "X264", "X265", "AV1"];
        const featureList = ["无码", "流出", "中文字幕", "字幕", "VR", "60FPS"];

        const mainTags: string[] = [];
        const endTags: string[] = [];

        rawTags.forEach((t: string) => {
            const upper = t.toUpperCase();
            const isResolution = resolutionList.some(r => upper.includes(r));
            const isCodec = codecList.some(c => upper.includes(c));
            const isFeature = featureList.includes(t) || featureList.some(f => upper.includes(f));

            if (isResolution || isCodec || isFeature) {
                endTags.push(t);
            } else {
                mainTags.push(t);
            }
        });

        return [...mainTags, ...endTags];
    };

    // 简介清洗
    const cleanOverview = (text: string) => {
        if (!text) return "暂无简介";
        return text
            .replace(/<!\[CDATA\[/g, '')
            .replace(/\]\]>/g, '')
            .replace(/<br\s*\/?>/g, '\n')
            .replace(/<[^>]+>/g, '')
            .trim();
    };

    const formatActorName = (name: string) => {
        if (!name) return "";
        // 过滤掉如 "明里?" 这种末尾问号异常值
        return name.replace(/\?+$/, '').replace(/\(\d+\)$/, '').trim();
    };

    const posterUrl = detail.poster_path ? `/local/${detail.poster_path}` : '';
    const fanartUrl = detail.backdrop_path ? `/local/${detail.backdrop_path}` : (detail.poster_path ? `/local/${detail.poster_path}` : '');

    const tags = getSortedTags();
    const actors = detail.actor ? detail.actor.split(/[,，/]/).map((s: string) => s.trim()).filter(Boolean) : [];
    const filename = currFilePath?.split(/[\\/]/).pop() || "未知文件";

    return (
        <div className="detail-workspace">
            <div className="detail-backdrop" style={{ backgroundImage: fanartUrl ? `url(${fanartUrl})` : 'none' }}></div>
            <div className="detail-backdrop-overlay"></div>

            <div className="detail-main">
                <div className="detail-poster-section">
                    {posterUrl ? (
                        <img src={posterUrl} className="detail-poster" alt="poster" />
                    ) : (
                        <div className="detail-poster no-poster">No Poster</div>
                    )}
                </div>

                <div className="detail-info-section">
                    <div className="detail-header-row">
                        <div className="detail-title">{detail.title}</div>
                        {msg && <span className="detail-status-msg">{msg}</span>}
                    </div>

                    {/* 工具条：精修顺序：删除 -> 目录 -> NFO -> 播放 -> 返回 -> 收藏/已看 */}
                    <div className="detail-toolbar">
                        <button className="toolbar-btn danger" title="注：仅从数据库移除此记录，不删除本地文件" onClick={handleDelete}><Trash2 size={16} /></button>
                        <button className="toolbar-btn" title="打开文件所在目录" onClick={handleOpenDir}><FolderOpen size={16} /></button>
                        <button className="toolbar-btn" title="编辑 NFO (修改保存后请手动刷新界面)" onClick={handleOpenNFO}><FileEdit size={16} /></button>
                        <button className="toolbar-btn primary" title="点击播放当前选择文件" onClick={handlePlay}><Play size={16} fill="currentColor" /></button>
                        <button className="toolbar-btn" title="返回主列表" onClick={onClose}><ArrowLeft size={16} /></button>
                        <div className="toolbar-divider"></div>
                        <button className="toolbar-btn" title={detail.is_favorite ? "取消收藏" : "收藏"} onClick={handleFav}>
                            <Star size={16} fill={detail.is_favorite ? "var(--accent)" : "none"} color={detail.is_favorite ? "var(--accent)" : "currentColor"} />
                        </button>
                        <button className="toolbar-btn" title={detail.is_watched ? "标记未看" : "标记已看"} onClick={handleWatched}>
                            {detail.is_watched ? <EyeOff size={16} /> : <Eye size={16} />}
                        </button>
                    </div>

                    <div className="detail-file-row">
                        <div className="detail-file-select" onClick={() => setShowFileMenu(!showFileMenu)}>
                            <div className="file-active-name" title={currFilePath}>{filename}</div>
                            <ChevronDown size={14} className={`chevron ${showFileMenu ? 'open' : ''}`} />
                            {showFileMenu && (
                                <div className="file-dropdown-menu">
                                    {files.map((f, i) => (
                                        <div key={i} className={`file-menu-item ${f === currFilePath ? 'active' : ''}`}
                                            onClick={(e) => { e.stopPropagation(); setCurrFilePath(f); setShowFileMenu(false); }}>
                                            <span className="file-item-name">{f.split(/[\\/]/).pop()}</span>
                                            {f === currFilePath && <Check size={12} color="var(--accent)" />}
                                        </div>
                                    ))}
                                </div>
                            )}
                        </div>
                    </div>

                    {/* 元数据网格精修 */}
                    <div className="detail-meta-grid">
                        <div className="meta-row">
                            <span className="meta-label">编号</span>
                            <span className="meta-value highlight">{getCode()}</span>
                        </div>
                        <div className="meta-row">
                            <span className="meta-label">日期</span>
                            <span className="meta-value">{detail.release_date_normalized || detail.year || "未知"}</span>
                        </div>
                        <div className="meta-row">
                            <span className="meta-label">时长</span>
                            <span className="meta-value">{detail.duration ? `${Math.floor(detail.duration / 60)} min` : (detail.runtime ? `${detail.runtime} min` : "未知")}</span>
                        </div>
                        <div className="meta-row">
                            <span className="meta-label">演员</span>
                            <div className="meta-value actor-list-inline">
                                {actors.length > 0 ? actors.map((a: string, idx: number) => {
                                    const name = formatActorName(a);
                                    return (
                                        <span key={idx} className="meta-actor-clickable" onClick={() => onSelectFilter({ type: 'actor', value: name, label: name })}>
                                            {name}{idx < actors.length - 1 ? " / " : ""}
                                        </span>
                                    );
                                }) : "未知"}
                            </div>
                        </div>
                        <div className="meta-row">
                            <span className="meta-label">类型</span>
                            <div className="meta-value tag-chips-scroll">
                                {tags.length > 0 ? tags.map((t: string, i: number) => (
                                    <span key={i} className="meta-pill-chip" onClick={() => onSelectFilter({ type: 'genre', value: t, label: t })}>{t}</span>
                                )) : "未分类"}
                            </div>
                        </div>
                        {detail.series?.title && (
                            <div className="meta-row">
                                <span className="meta-label">系列</span>
                                <span className="meta-value meta-item-clickable" onClick={() => onSelectFilter({ type: 'series', value: detail.series.id, label: detail.series.title })}>
                                    {detail.series.title}
                                </span>
                            </div>
                        )}
                        {(detail.studio || detail.publisher) && (
                            <div className="meta-row">
                                <span className="meta-label">发行</span>
                                <span className="meta-value">{detail.studio || detail.publisher}</span>
                            </div>
                        )}
                    </div>

                    <div className="detail-desc">
                        {cleanOverview(detail.overview)}
                    </div>

                    {previews.length > 0 && (
                        <div className="detail-previews-container">
                            <div className="previews-label">预览剧照 ({previews.length})</div>
                            <div className="preview-strip">
                                {previews.map((p, i) => (
                                    <div key={i} className="preview-item">
                                        <img src={`/local/${p}`} className="preview-img" alt="preview" loading="lazy" />
                                    </div>
                                ))}
                            </div>
                        </div>
                    )}
                </div>
            </div>
        </div>
    );
};

export default MediaDetail;
