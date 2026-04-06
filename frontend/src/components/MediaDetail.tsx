import React, { useEffect, useRef, useState } from 'react';
import {
    DeleteMedia,
    GetMediaDetail,
    GetMediaFiles,
    GetMediaPreviews,
    GetNFOEditorData,
    OpenMediaFolder,
    PlayFile,
    SaveNFOEditorData,
    ToggleFavorite,
    ToggleWatched,
} from "../../wailsjs/go/main/App";
import {
    ArrowLeft,
    Check,
    ChevronDown,
    Eye,
    EyeOff,
    FileEdit,
    FolderOpen,
    Play,
    Star,
    Trash2,
} from 'lucide-react';
import NFOEditModal from './NFOEditModal';

interface MediaDetailProps {
    media: any;
    onClose: () => void;
    onSelectFilter: (filter: { type: string; value: string; label: string }) => void;
}

interface DetailActor {
    id?: string;
    name: string;
}

type DetailImageRole = 'poster' | 'backdrop';

const imageTokenPattern = /[-_.\s]+/;

const getImageTokens = (path: string) => {
    const filename = path.split(/[\\/]/).pop()?.toLowerCase() || '';
    const stem = filename.replace(/\.[^.]+$/, '');
    return stem.split(imageTokenPattern).filter(Boolean);
};

const hasImageToken = (path: string, token: string) => getImageTokens(path).includes(token);

const pickDetailImagePath = (detail: any, previews: string[], role: DetailImageRole) => {
    const coverTokens = ['cover', 'folder', 'thumb', 'movie', 'show'];
    const backgroundTokens = ['fanart', 'backdrop', 'background', 'banner', 'clearart', 'landscape'];
    const candidates = Array.from(new Set(
        [detail.poster_path, detail.backdrop_path, detail.fanart_path, ...previews]
            .filter((path): path is string => typeof path === 'string' && path.trim().length > 0),
    ));

    const isPosterImage = (path: string) => hasImageToken(path, 'poster');
    const isCoverLikeImage = (path: string) => coverTokens.some((token) => hasImageToken(path, token));
    const isBackgroundImage = (path: string) => backgroundTokens.some((token) => hasImageToken(path, token));

    const ranked = candidates
        .map((path, index) => {
            let priority = Number.POSITIVE_INFINITY;

            if (role === 'poster') {
                if (isPosterImage(path)) priority = 0;
                else if (isCoverLikeImage(path)) priority = 1;
                else if (path === detail.poster_path && !isBackgroundImage(path)) priority = 2;
            } else {
                if (hasImageToken(path, 'fanart')) priority = 0;
                else if (hasImageToken(path, 'backdrop')) priority = 1;
                else if (['background', 'banner', 'clearart', 'landscape'].some((token) => hasImageToken(path, token))) priority = 2;
                else if (path === detail.backdrop_path && !isPosterImage(path) && !isCoverLikeImage(path)) priority = 3;
            }

            return { path, index, priority };
        })
        .filter((candidate) => Number.isFinite(candidate.priority))
        .sort((left, right) => left.priority - right.priority || left.index - right.index);

    return ranked[0]?.path || '';
};

const formatError = (error: unknown) => {
    if (error instanceof Error && error.message) {
        return error.message;
    }
    if (typeof error === 'string') {
        return error;
    }
    return '未知错误';
};

const formatActorName = (name: string) => {
    if (!name) {
        return '';
    }
    return name.replace(/[?？\s]+$/, '').replace(/\(\d+\)$/, '').trim();
};

const cleanOverview = (text: string) => {
    if (!text) {
        return '暂无简介';
    }
    return text
        .replace(/<!\[CDATA\[/g, '')
        .replace(/\]\]>/g, '')
        .replace(/<br\s*\/?>/gi, '\n')
        .replace(/<[^>]+>/g, '')
        .trim();
};

const getMediaCode = (detail: any, currFilePath: string) => {
    if (detail.nfo_extra_fields) {
        try {
            const extra = JSON.parse(detail.nfo_extra_fields);
            if (extra.num) {
                return String(extra.num).toUpperCase();
            }
        } catch (_error) {
            // ignore malformed nfo_extra_fields and keep fallback chain below
        }
    }

    const filename = currFilePath?.split(/[\\/]/).pop() || '';
    const codeMatch = filename.match(/([A-Z0-9]{2,10}-\d{2,6})/i);
    if (codeMatch) {
        return codeMatch[1].toUpperCase();
    }

    return detail.code || detail.id?.slice(0, 8) || '未知';
};

const normalizeActors = (detail: any): DetailActor[] => {
    if (Array.isArray(detail.actors) && detail.actors.length > 0) {
        return detail.actors
            .map((actor: any) => ({
                id: actor?.id,
                name: formatActorName(actor?.name || ''),
            }))
            .filter((actor: DetailActor) => actor.name);
    }

    if (detail.actor) {
        return String(detail.actor)
            .split(/[,，/]/)
            .map((name: string) => ({ name: formatActorName(name) }))
            .filter((actor: DetailActor) => actor.name);
    }

    return [];
};

const normalizeTags = (detail: any) => {
    const rawTags = detail.genres
        ? String(detail.genres).split(/[,，/]/).map((tag: string) => tag.trim()).filter(Boolean)
        : [];

    const technicalKeywords = [
        '4K', '1080P', '720P', 'UHD', 'HD', 'FHD', 'SD',
        'H265', 'HEVC', 'H264', 'X264', 'X265', 'AV1', 'HDR',
        '中文字幕', '字幕', '60FPS', 'FPS', '无码', '流出', 'REMUX', 'WEB-DL',
    ];
    const coreKeywords = [
        '剧情', '恋爱', '人妻', '素人', '学生', '老师', '护士', '秘书', 'OL',
        '校园', '职场', '旅行', '温泉', '家庭', '情侣', '制服', '巨乳',
        '熟女', '姐姐', '妹妹', '偶像', '角色', '人物',
    ];

    const seen = new Set<string>();
    const uniqueTags = rawTags.filter((tag: string) => {
        const key = tag.toLowerCase();
        if (seen.has(key)) {
            return false;
        }
        seen.add(key);
        return true;
    });

    const scoreTag = (tag: string) => {
        const upper = tag.toUpperCase();
        const isTechnical = technicalKeywords.some((keyword) => upper.includes(keyword) || tag.includes(keyword));
        if (isTechnical) {
            return 200 + tag.length;
        }
        const isCore = coreKeywords.some((keyword) => tag.includes(keyword));
        if (isCore) {
            return tag.length;
        }
        return 80 + tag.length;
    };

    return uniqueTags.sort((left: string, right: string) => scoreTag(left) - scoreTag(right));
};

const MediaDetail: React.FC<MediaDetailProps> = ({ media, onClose, onSelectFilter }) => {
    const [detail, setDetail] = useState(media);
    const [msg, setMsg] = useState('');
    const [files, setFiles] = useState<string[]>([]);
    const [previews, setPreviews] = useState<string[]>([]);
    const [currFilePath, setCurrFilePath] = useState(media.file_path || '');
    const [showFileMenu, setShowFileMenu] = useState(false);
    const [showNFOEditor, setShowNFOEditor] = useState(false);
    const [nfoEditorData, setNfoEditorData] = useState<any>(null);
    const [nfoLoading, setNfoLoading] = useState(false);
    const [nfoSaving, setNfoSaving] = useState(false);
    const fileDropdownRef = useRef<HTMLDivElement | null>(null);

    const showMsg = (message: string) => {
        setMsg(message);
        window.setTimeout(() => setMsg(''), 4000);
    };

    useEffect(() => {
        const load = async () => {
            try {
                const [nextDetail, nextFiles, nextPreviews] = await Promise.all([
                    GetMediaDetail(media.id),
                    GetMediaFiles(media.id),
                    GetMediaPreviews(media.id),
                ]);

                const normalizedFiles = Array.isArray(nextFiles) ? nextFiles : [];
                const initialFilePath = (nextDetail?.file_path || normalizedFiles[0] || media.file_path || '').trim();

                setDetail(nextDetail);
                setFiles(normalizedFiles);
                setPreviews(Array.isArray(nextPreviews) ? nextPreviews : []);
                setCurrFilePath(initialFilePath);
            } catch (error) {
                console.error(error);
                showMsg(`加载详情失败：${formatError(error)}`);
            }
        };

        load();
    }, [media.id, media.file_path]);

    useEffect(() => {
        const onPointerDown = (event: MouseEvent) => {
            if (!fileDropdownRef.current?.contains(event.target as Node)) {
                setShowFileMenu(false);
            }
        };

        const onEscape = (event: KeyboardEvent) => {
            if (event.key === 'Escape') {
                setShowFileMenu(false);
            }
        };

        document.addEventListener('mousedown', onPointerDown);
        document.addEventListener('keydown', onEscape);
        return () => {
            document.removeEventListener('mousedown', onPointerDown);
            document.removeEventListener('keydown', onEscape);
        };
    }, []);

    const refreshDetail = async () => {
        const nextDetail = await GetMediaDetail(media.id);
        setDetail(nextDetail);
    };

    const handlePlay = async () => {
        const targetPath = (currFilePath || detail.file_path || media.file_path || '').trim();
        if (!targetPath) {
            showMsg('播放失败：当前没有可播放文件');
            return;
        }

        try {
            showMsg(`正在启动播放器：${targetPath.split(/[\\/]/).pop()}`);
            await PlayFile(targetPath);
            await refreshDetail();
        } catch (error) {
            console.error(error);
            showMsg(`播放失败：${formatError(error)}`);
        }
    };

    const handleOpenDir = async () => {
        try {
            await OpenMediaFolder(detail.id);
        } catch (error) {
            console.error(error);
            showMsg(`打开目录失败：${formatError(error)}`);
        }
    };

    const handleOpenNFO = async () => {
        try {
            setShowNFOEditor(true);
            setNfoLoading(true);
            const nextData = await GetNFOEditorData(detail.id);
            setNfoEditorData(nextData);
        } catch (error) {
            console.error(error);
            setShowNFOEditor(false);
            showMsg(`打开 NFO 失败：${formatError(error)}`);
        } finally {
            setNfoLoading(false);
        }
    };

    const handleSaveNFO = async (draft: any) => {
        try {
            setNfoSaving(true);
            await SaveNFOEditorData(detail.id, draft);
            await refreshDetail();
            setNfoEditorData(draft);
            setShowNFOEditor(false);
            showMsg('NFO 已保存');
        } catch (error) {
            console.error(error);
            showMsg(`保存 NFO 失败：${formatError(error)}`);
        } finally {
            setNfoSaving(false);
        }
    };

    const handleDelete = async () => {
        if (!window.confirm('确定要从数据库中移除此条目吗？注意：不会删除本地文件。')) {
            return;
        }

        try {
            await DeleteMedia(detail.id);
            onClose();
        } catch (error) {
            console.error(error);
            showMsg(`删除失败：${formatError(error)}`);
        }
    };

    const handleFav = async () => {
        try {
            await ToggleFavorite(detail.id);
            setDetail((prev: any) => ({ ...prev, is_favorite: !prev.is_favorite }));
            showMsg('收藏状态已更新');
        } catch (error) {
            console.error(error);
            showMsg(`收藏失败：${formatError(error)}`);
        }
    };

    const handleWatched = async () => {
        try {
            await ToggleWatched(detail.id);
            setDetail((prev: any) => ({ ...prev, is_watched: !prev.is_watched }));
            showMsg('观看状态已更新');
        } catch (error) {
            console.error(error);
            showMsg(`更新观看状态失败：${formatError(error)}`);
        }
    };

    const handleChipStripWheel = (event: React.WheelEvent<HTMLDivElement>) => {
        const container = event.currentTarget;
        if (container.scrollWidth <= container.clientWidth) {
            return;
        }

        const delta = Math.abs(event.deltaX) > Math.abs(event.deltaY) ? event.deltaX : event.deltaY;
        if (delta === 0) {
            return;
        }

        container.scrollLeft += delta;
        event.preventDefault();
    };

    const posterPath = pickDetailImagePath(detail, previews, 'poster');
    const backdropPath = pickDetailImagePath(detail, previews, 'backdrop') || posterPath;
    const posterUrl = posterPath ? `/local/${posterPath}` : '';
    const backdropUrl = backdropPath ? `/local/${backdropPath}` : '';
    const actors = normalizeActors(detail);
    const tags = normalizeTags(detail);
    const filename = currFilePath?.split(/[\\/]/).pop() || '未知文件';

    return (
        <>
            <div className="detail-workspace">
                <div className="detail-backdrop-layer" aria-hidden="true">
                    {backdropUrl && (
                        <img
                            src={backdropUrl}
                            className="detail-backdrop-image"
                            alt=""
                        />
                    )}
                    <div className="detail-backdrop-overlay" />
                </div>

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

                        <div className="detail-toolbar">
                            <button className="toolbar-btn danger" title="仅从数据库移除这条记录，不删除本地文件" onClick={handleDelete}><Trash2 size={16} /></button>
                            <button className="toolbar-btn" title="打开文件所在目录" onClick={handleOpenDir}><FolderOpen size={16} /></button>
                            <button className="toolbar-btn" title="编辑 NFO" onClick={handleOpenNFO}><FileEdit size={16} /></button>
                            <button className="toolbar-btn primary" title="播放当前选中文件" onClick={handlePlay}><Play size={16} fill="currentColor" /></button>
                            <button className="toolbar-btn" title="返回列表" onClick={onClose}><ArrowLeft size={16} /></button>
                            <div className="toolbar-divider" />
                            <button className="toolbar-btn" title={detail.is_favorite ? '取消收藏' : '收藏'} onClick={handleFav}>
                                <Star size={16} fill={detail.is_favorite ? "var(--accent)" : "none"} color={detail.is_favorite ? "var(--accent)" : "currentColor"} />
                            </button>
                            <button className="toolbar-btn" title={detail.is_watched ? '标记未看' : '标记已看'} onClick={handleWatched}>
                                {detail.is_watched ? <EyeOff size={16} /> : <Eye size={16} />}
                            </button>
                        </div>

                        <div className="detail-file-row">
                            <div className="detail-file-dropdown" ref={fileDropdownRef}>
                                <button
                                    type="button"
                                    className={`detail-file-select ${showFileMenu ? 'open' : ''}`}
                                    onClick={() => setShowFileMenu((open) => !open)}
                                >
                                    <span className="file-active-name" title={currFilePath}>{filename}</span>
                                    <ChevronDown size={14} className={`chevron ${showFileMenu ? 'open' : ''}`} />
                                </button>

                                {showFileMenu && (
                                    <div className="file-dropdown-menu">
                                        {files.map((file, index) => (
                                            <button
                                                key={`${file}-${index}`}
                                                type="button"
                                                className={`file-menu-item ${file === currFilePath ? 'active' : ''}`}
                                                onClick={() => {
                                                    setCurrFilePath(file);
                                                    setShowFileMenu(false);
                                                }}
                                            >
                                                <span className="file-item-name">{file.split(/[\\/]/).pop()}</span>
                                                {file === currFilePath && <Check size={12} color="var(--accent)" />}
                                            </button>
                                        ))}
                                    </div>
                                )}
                            </div>
                        </div>

                        <div className="detail-meta-grid">
                            <div className="meta-row">
                                <span className="meta-label">编号</span>
                                <span className="meta-value highlight">{getMediaCode(detail, currFilePath)}</span>
                            </div>
                            <div className="meta-row">
                                <span className="meta-label">日期</span>
                                <span className="meta-value">{detail.release_date_normalized || detail.year || '未知'}</span>
                            </div>
                            <div className="meta-row">
                                <span className="meta-label">时长</span>
                                <span className="meta-value">
                                    {detail.duration ? `${Math.floor(detail.duration / 60)} min` : (detail.runtime ? `${detail.runtime} min` : '未知')}
                                </span>
                            </div>
                            <div className="meta-row chips-row">
                                <span className="meta-label">演员</span>
                                <div className="meta-value actor-chip-strip">
                                    {actors.length > 0 ? actors.map((actor: DetailActor, index: number) => (
                                        <button
                                            type="button"
                                            key={actor.id || `${actor.name}-${index}`}
                                            className="meta-pill-chip actor-pill-chip"
                                            onClick={() => onSelectFilter({ type: 'actor', value: actor.id || actor.name, label: actor.name })}
                                        >
                                            {actor.name}
                                        </button>
                                    )) : '未知'}
                                </div>
                            </div>
                            <div className="meta-row chips-row">
                                <span className="meta-label">类型</span>
                                <div className="meta-value chip-strip-shell" onWheel={handleChipStripWheel}>
                                    <div className="tag-chips-scroll">
                                        {tags.length > 0 ? tags.map((tag: string, index: number) => (
                                            <button
                                                key={`${tag}-${index}`}
                                                type="button"
                                                className="meta-pill-chip"
                                                onClick={() => onSelectFilter({ type: 'genre', value: tag, label: tag })}
                                            >
                                                {tag}
                                            </button>
                                        )) : '未分类'}
                                    </div>
                                </div>
                            </div>
                            {detail.series?.title && (
                                <div className="meta-row">
                                    <span className="meta-label">系列</span>
                                    <span
                                        className="meta-value meta-item-clickable"
                                        onClick={() => onSelectFilter({ type: 'series', value: detail.series.id, label: detail.series.title })}
                                    >
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
                                    {previews.map((preview, index) => (
                                        <div key={`${preview}-${index}`} className="preview-item">
                                            <img src={`/local/${preview}`} className="preview-img" alt="preview" loading="lazy" />
                                        </div>
                                    ))}
                                </div>
                            </div>
                        )}
                    </div>
                </div>
            </div>

            {showNFOEditor && (
                <NFOEditModal
                    data={nfoEditorData}
                    loading={nfoLoading}
                    saving={nfoSaving}
                    onClose={() => !nfoSaving && setShowNFOEditor(false)}
                    onSave={handleSaveNFO}
                />
            )}
        </>
    );
};

export default MediaDetail;
