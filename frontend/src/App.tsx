import { useEffect, useRef, useState } from 'react';
import './App.css';
import {
    GetActorStats,
    GetDesktopSettings,
    GetDirectoryStats,
    GetGenreStats,
    GetLibraries,
    GetSeriesStats,
    PlayRandomLibraryMedia,
    ScanLibraryWithMode,
} from "../wailsjs/go/main/App";
import { EventsOn, WindowSetTitle } from "../wailsjs/runtime/runtime";
import Sidebar from './components/Sidebar';
import TopBar from './components/TopBar';
import MediaGrid from './components/MediaGrid';
import CategoryGrid from './components/CategoryGrid';
import SettingsPage from './components/SettingsPage';
import LibraryModal from './components/LibraryModal';
import LibraryEditModal from './components/LibraryEditModal';
import MediaDetail from './components/MediaDetail';

type ViewName = 'libs' | 'settings' | 'directory' | 'actor' | 'genre' | 'series' | 'watched' | 'favorite';
type SortField = 'created_at' | 'release_date' | 'video_codec' | 'last_watched';
type SortOrder = 'asc' | 'desc';
type FilterState = { type: string; value: string; label: string } | null;
type FilterReturnContext = {
    view: ViewName;
    media: any | null;
} | null;

const APP_TITLE = 'ALEX';

const SCAN_MODE_LABELS: Record<string, string> = {
    overwrite: '覆盖刷新',
    delete_update: '删改刷新',
    incremental: '新增刷新',
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

function App() {
    const [view, setView] = useState<ViewName>('libs');
    const [libraries, setLibraries] = useState<any[]>([]);
    const [currentLib, setCurrentLib] = useState<any>(null);
    const [searchKeyword, setSearchKeyword] = useState('');
    const [showLibModal, setShowLibModal] = useState(false);
    const [editingLib, setEditingLib] = useState<any>(null);
    const [selectedMedia, setSelectedMedia] = useState<any>(null);
    const [statusMsg, setStatusMsg] = useState('');
    const [mediaCount, setMediaCount] = useState(0);
    const [activeFilter, setActiveFilter] = useState<FilterState>(null);
    const [filterReturnContext, setFilterReturnContext] = useState<FilterReturnContext>(null);
    const [sortField, setSortField] = useState<SortField>('created_at');
    const [sortOrder, setSortOrder] = useState<SortOrder>('desc');
    const scanStartedAtRef = useRef<number | null>(null);
    const resetTitleTimerRef = useRef<number | null>(null);

    const setAppTitle = (title: string) => {
        WindowSetTitle(title);
    };

    const scheduleTitleReset = (delay = 3500) => {
        if (resetTitleTimerRef.current) {
            window.clearTimeout(resetTitleTimerRef.current);
        }
        resetTitleTimerRef.current = window.setTimeout(() => {
            setAppTitle(APP_TITLE);
            resetTitleTimerRef.current = null;
        }, delay);
    };

    const updateScanTitle = (data: any, fallbackPrefix: string) => {
        const current = typeof data?.current === 'number' ? data.current : 0;
        const total = typeof data?.total === 'number' ? data.total : 0;
        const elapsedSeconds = scanStartedAtRef.current ? Math.max(0, Math.floor((Date.now() - scanStartedAtRef.current) / 1000)) : 0;
        const ratioText = total > 0 ? `${current}/${total}` : `${current}`;
        const message = typeof data?.message === 'string' ? data.message.trim() : '';
        const suffix = elapsedSeconds > 0 ? `，耗时: ${elapsedSeconds}秒` : '';
        const detail = message ? ` ${message}` : '';
        setAppTitle(`${APP_TITLE} - ${fallbackPrefix}${ratioText}${detail}${suffix}`);
    };

    const showStatus = (msg: string) => {
        setStatusMsg(msg);
        window.setTimeout(() => setStatusMsg(''), 5000);
    };

    const loadLibraries = async () => {
        try {
            const libs = await GetLibraries();
            const nextLibraries = libs || [];
            setLibraries(nextLibraries);
            setCurrentLib((prev: any) => {
                if (nextLibraries.length === 0) {
                    return null;
                }
                if (!prev) {
                    return nextLibraries[0];
                }
                return nextLibraries.find((lib: any) => lib.id === prev.id) || nextLibraries[0];
            });
            setEditingLib((prev: any) => {
                if (!prev) {
                    return prev;
                }
                return nextLibraries.find((lib: any) => lib.id === prev.id) || null;
            });
        } catch (error) {
            console.error(error);
        }
    };

    useEffect(() => {
        setAppTitle(APP_TITLE);
        loadLibraries();

        GetDesktopSettings().then((settings: any) => {
            if (settings?.theme) {
                document.body.className = settings.theme;
            }
        });

        const unsubStart = EventsOn("scan:started", (data: any) => {
            scanStartedAtRef.current = Date.now();
            setStatusMsg(`扫描开始：${data?.library_name || ''}`);
            updateScanTitle(data, '扫描 ');
        });

        const unsubProgress = EventsOn("scan:progress", (data: any) => {
            setStatusMsg(`扫描中：${data?.message || ''} [${data?.current || 0}/${data?.total || 0}]`);
            updateScanTitle(data, '扫描 ');
        });

        const unsubComplete = EventsOn("scan:completed", (data: any) => {
            showStatus(`扫描完成：${data?.library_name || ''}`);
            updateScanTitle(data, '完成 ');
            scanStartedAtRef.current = null;
            scheduleTitleReset();
            loadLibraries();
        });

        const unsubFail = EventsOn("scan:failed", (data: any) => {
            showStatus(`扫描失败：${data?.message || '未知错误'}`);
            updateScanTitle(data, '失败 ');
            scanStartedAtRef.current = null;
            scheduleTitleReset();
        });

        return () => {
            unsubStart();
            unsubProgress();
            unsubComplete();
            unsubFail();
            if (resetTitleTimerRef.current) {
                window.clearTimeout(resetTitleTimerRef.current);
            }
            setAppTitle(APP_TITLE);
        };
    }, []);

    const clearFilter = () => {
        setActiveFilter(null);
        setSearchKeyword('');
        if (filterReturnContext) {
            setView(filterReturnContext.view);
            setSelectedMedia(filterReturnContext.media);
        } else {
            setView('libs');
            setSelectedMedia(null);
        }
        setFilterReturnContext(null);
    };

    const applyFilter = (filter: { type: string; value: string; label: string }, returnContext?: FilterReturnContext) => {
        setActiveFilter(filter);
        setSearchKeyword(filter.label);
        setFilterReturnContext(returnContext || null);
        setSelectedMedia(null);
        setView('libs');
    };

    const applyFilterFromView = (sourceView: ViewName, filter: { type: string; value: string; label: string }) => {
        applyFilter(filter, { view: sourceView, media: null });
    };

    const applyFilterFromDetail = (filter: { type: string; value: string; label: string }) => {
        applyFilter(filter, {
            view,
            media: selectedMedia,
        });
    };

    const handleSelectLibrary = (lib: any) => {
        setCurrentLib(lib);
        setView('libs');
        setSelectedMedia(null);
        setActiveFilter(null);
        setFilterReturnContext(null);
        setSearchKeyword('');
    };

    const handleOpenSettings = () => {
        setSelectedMedia(null);
        setActiveFilter(null);
        setFilterReturnContext(null);
        setSearchKeyword('');
        setView('settings');
    };

    const handleSelectView = (nextView: ViewName) => {
        setSelectedMedia(null);
        setView(nextView);
        setActiveFilter(null);
        setFilterReturnContext(null);
        setSearchKeyword('');
    };

    const handleLibCreated = () => {
        setShowLibModal(false);
        loadLibraries();
        showStatus('新建媒体库成功');
    };

    const handleLibSaved = () => {
        setEditingLib(null);
        loadLibraries();
        showStatus('媒体库已保存');
    };

    const handleLibDeleted = () => {
        setEditingLib(null);
        setCurrentLib(null);
        loadLibraries();
        showStatus('媒体库已删除');
    };

    const handleScanWithMode = async (mode: string) => {
        if (!currentLib) {
            return;
        }
        try {
            await ScanLibraryWithMode(currentLib.id, mode);
            showStatus(`${SCAN_MODE_LABELS[mode] || mode}已启动`);
        } catch (error) {
            showStatus(`扫描启动失败：${formatError(error)}`);
        }
    };

    const handleRandomPlay = async () => {
        if (!currentLib) {
            return;
        }
        try {
            const filename = await PlayRandomLibraryMedia(currentLib.id);
            showStatus(`随机播放：${filename}`);
        } catch (error) {
            showStatus(`随机播放失败：${formatError(error)}`);
        }
    };

    const handleSortSelect = (field: string) => {
        const nextField = field as SortField;
        if (nextField === sortField) {
            setSortOrder((prev) => prev === 'desc' ? 'asc' : 'desc');
            return;
        }
        setSortField(nextField);
        setSortOrder('desc');
    };

    const renderLibraryTopBar = (name: string, count: number, showControls: boolean) => (
        <TopBar
            libName={name}
            mediaCount={count}
            searchValue={searchKeyword}
            onSearch={setSearchKeyword}
            onScanWithMode={handleScanWithMode}
            onEditLibrary={showControls && currentLib ? () => setEditingLib(currentLib) : undefined}
            onRandomPlay={showControls && currentLib ? handleRandomPlay : undefined}
            onSortSelect={showControls ? handleSortSelect : undefined}
            sortField={sortField}
            sortOrder={sortOrder}
            showLibraryControls={showControls}
        />
    );

    return (
        <div className="app-container">
            <Sidebar
                libraries={libraries}
                currentLib={currentLib}
                currentView={view}
                onSelectLib={handleSelectLibrary}
                onOpenSettings={handleOpenSettings}
                onSelectView={handleSelectView}
                onAddLib={() => setShowLibModal(true)}
                onEditLib={(lib: any) => setEditingLib(lib)}
            />

            <div className="main-content">
                {view === 'settings' ? (
                    <SettingsPage onClose={() => setView('libs')} />
                ) : selectedMedia ? (
                    <MediaDetail
                        media={selectedMedia}
                        onClose={() => setSelectedMedia(null)}
                        onSelectFilter={applyFilterFromDetail}
                    />
                ) : (
                    <>
                        {view === 'libs' && currentLib && (
                            <>
                                {renderLibraryTopBar(currentLib.name, mediaCount, true)}
                                {activeFilter && (
                                    <div className="filter-bar">
                                        <span>当前筛选：{activeFilter.label}</span>
                                        <button className="filter-clear" onClick={clearFilter}>清除筛选</button>
                                    </div>
                                )}
                                <MediaGrid
                                    libraryId={currentLib.id}
                                    keyword={searchKeyword}
                                    sortField={sortField}
                                    sortOrder={sortOrder}
                                    filter={activeFilter}
                                    onSelectMedia={setSelectedMedia}
                                    onCountChange={setMediaCount}
                                    onQuickPlayStatus={showStatus}
                                />
                            </>
                        )}

                        {view === 'watched' && currentLib && (
                            <>
                                {renderLibraryTopBar('已看列表', mediaCount, false)}
                                <div className="filter-bar">
                                    <span>当前视图：已看 / 历史记录</span>
                                    <button className="filter-clear" onClick={() => setView('libs')}>返回主页</button>
                                </div>
                                <MediaGrid
                                    libraryId={currentLib.id}
                                    keyword={searchKeyword}
                                    sortField={sortField}
                                    sortOrder={sortOrder}
                                    filter={{ type: 'watched', value: 'true', label: '已看' }}
                                    onSelectMedia={setSelectedMedia}
                                    onCountChange={setMediaCount}
                                />
                            </>
                        )}

                        {view === 'favorite' && currentLib && (
                            <>
                                {renderLibraryTopBar('我的收藏', mediaCount, false)}
                                <div className="filter-bar">
                                    <span>当前视图：我的收藏</span>
                                    <button className="filter-clear" onClick={() => setView('libs')}>返回主页</button>
                                </div>
                                <MediaGrid
                                    libraryId={currentLib.id}
                                    keyword={searchKeyword}
                                    sortField={sortField}
                                    sortOrder={sortOrder}
                                    filter={{ type: 'favorite', value: 'true', label: '收藏' }}
                                    onSelectMedia={setSelectedMedia}
                                    onCountChange={setMediaCount}
                                />
                            </>
                        )}

                        {view === 'directory' && currentLib && (
                            <>
                                {renderLibraryTopBar('目录聚合', 0, false)}
                                <CategoryGrid
                                    type="directory"
                                    libraryId={currentLib.id}
                                    fetchFn={GetDirectoryStats}
                                    onSelect={(value, label) => applyFilterFromView('directory', { type: 'directory', value, label })}
                                />
                            </>
                        )}

                        {view === 'actor' && currentLib && (
                            <>
                                {renderLibraryTopBar('演员', 0, false)}
                                <CategoryGrid
                                    type="actor"
                                    libraryId={currentLib.id}
                                    fetchFn={GetActorStats}
                                    onSelect={(value, label) => applyFilterFromView('actor', { type: 'actor', value, label })}
                                />
                            </>
                        )}

                        {view === 'genre' && currentLib && (
                            <>
                                {renderLibraryTopBar('类别统计', 0, false)}
                                <CategoryGrid
                                    type="genre"
                                    libraryId={currentLib.id}
                                    fetchFn={GetGenreStats}
                                    onSelect={(value, label) => applyFilterFromView('genre', { type: 'genre', value, label })}
                                />
                            </>
                        )}

                        {view === 'series' && currentLib && (
                            <>
                                {renderLibraryTopBar('系列 / 集合', 0, false)}
                                <CategoryGrid
                                    type="series"
                                    libraryId={currentLib.id}
                                    fetchFn={GetSeriesStats}
                                    onSelect={(value, label) => applyFilterFromView('series', { type: 'series', value, label })}
                                />
                            </>
                        )}

                        {view === 'libs' && !currentLib && (
                            <div className="empty-state">
                                请先新建媒体库
                            </div>
                        )}
                    </>
                )}
            </div>

            {showLibModal && (
                <LibraryModal onClose={() => setShowLibModal(false)} onSuccess={handleLibCreated} />
            )}

            {editingLib && (
                <LibraryEditModal
                    library={editingLib}
                    onClose={() => setEditingLib(null)}
                    onSaved={handleLibSaved}
                    onDeleted={handleLibDeleted}
                />
            )}

            {statusMsg && (
                <div className="status-toast">{statusMsg}</div>
            )}
        </div>
    );
}

export default App;
