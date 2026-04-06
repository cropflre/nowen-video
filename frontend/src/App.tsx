import { useState, useEffect } from 'react';
import './App.css';
import { GetLibraries, GetDesktopSettings, ScanLibrary, ScanLibraryWithMode, GetDirectoryStats, GetActorStats, GetGenreStats, GetSeriesStats } from "../wailsjs/go/main/App";
import { EventsOn } from "../wailsjs/runtime/runtime";
import Sidebar from './components/Sidebar';
import TopBar from './components/TopBar';
import MediaGrid from './components/MediaGrid';
import CategoryGrid from './components/CategoryGrid';
import SettingsPage from './components/SettingsPage';
import LibraryModal from './components/LibraryModal';
import LibraryEditModal from './components/LibraryEditModal';
import MediaDetail from './components/MediaDetail';

function App() {
    const [view, setView] = useState<'libs' | 'settings' | 'directory' | 'actor' | 'genre' | 'series' | 'watched' | 'favorite'>('libs');
    const [libraries, setLibraries] = useState<any[]>([]);
    const [currentLib, setCurrentLib] = useState<any>(null);
    const [searchKeyword, setSearchKeyword] = useState('');
    const [showLibModal, setShowLibModal] = useState(false);
    const [editingLib, setEditingLib] = useState<any>(null);
    const [selectedMedia, setSelectedMedia] = useState<any>(null);
    const [statusMsg, setStatusMsg] = useState('');
    const [mediaCount, setMediaCount] = useState(0);
    const [activeFilter, setActiveFilter] = useState<{ type: string; value: string; label: string } | null>(null);

    const showStatus = (msg: string) => {
        setStatusMsg(msg);
        setTimeout(() => setStatusMsg(''), 5000);
    };

    const loadLibraries = async () => {
        try {
            const libs = await GetLibraries();
            setLibraries(libs || []);
            if (libs && libs.length > 0 && !currentLib) {
                setCurrentLib(libs[0]);
            }
        } catch (e) {
            console.error(e);
        }
    };

    useEffect(() => {
        loadLibraries();
        
        GetDesktopSettings().then((settings: any) => {
            if (settings && settings.theme) {
                document.body.className = settings.theme;
            }
        });

        const unsubStart = EventsOn("scan:started", (data: any) => {
            setStatusMsg(`扫描开始: ${data?.library_name}`);
        });
        const unsubProgress = EventsOn("scan:progress", (data: any) => {
            setStatusMsg(`扫描中: ${data?.message} [${data?.current}/${data?.total}]`);
        });
        const unsubComplete = EventsOn("scan:completed", (data: any) => {
            showStatus(`扫描完成: ${data?.library_name}`);
            loadLibraries();
        });
        const unsubFail = EventsOn("scan:failed", (data: any) => {
            showStatus(`扫描失败: ${data?.message}`);
        });

        return () => {
            unsubStart(); unsubProgress(); unsubComplete(); unsubFail();
        };
    }, []);

    const handleLibCreated = () => {
        setShowLibModal(false);
        loadLibraries();
        showStatus("新建媒体库成功");
    };

    const handleLibSaved = () => {
        setEditingLib(null);
        loadLibraries();
        showStatus("媒体库已保存");
    };

    const handleLibDeleted = () => {
        setEditingLib(null);
        setCurrentLib(null);
        loadLibraries();
        showStatus("媒体库已删除");
    };

    const handleScan = async () => {
        if (!currentLib) return;
        try {
            await ScanLibrary(currentLib.id);
        } catch(e) {
            showStatus(`扫描启动失败: ${e}`);
        }
    };

    const handleScanWithMode = async (mode: string) => {
        if (!currentLib) return;
        try {
            await ScanLibraryWithMode(currentLib.id, mode);
            showStatus(`${mode} 扫描已启动`);
        } catch(e) {
            showStatus(`扫描启动失败: ${e}`);
        }
    };

    return (
        <div className="app-container">
            <Sidebar 
                libraries={libraries} 
                currentLib={currentLib}
                currentView={view}
                onSelectLib={(lib: any) => { setCurrentLib(lib); setView('libs'); setActiveFilter(null); }}
                onOpenSettings={() => setView('settings')}
                onSelectView={(v: any) => setView(v)}
                onAddLib={() => setShowLibModal(true)}
                onEditLib={(lib: any) => setEditingLib(lib)}
            />
            
            <div className="main-content">
                {selectedMedia ? (
                    <MediaDetail 
                        media={selectedMedia} 
                        onClose={() => setSelectedMedia(null)} 
                        onSelectFilter={(filter: any) => {
                            setActiveFilter(filter);
                            setSelectedMedia(null);
                            setView('libs');
                        }}
                    />
                ) : (
                    <>
                        {(view === 'libs' || (view !== 'settings' && activeFilter)) && currentLib && (
                            <>
                                <TopBar 
                                    libName={currentLib.name}
                                    mediaCount={mediaCount}
                                    onSearch={setSearchKeyword} 
                                    onScan={handleScan}
                                    onScanWithMode={handleScanWithMode}
                                />
                                {activeFilter && (
                                    <div className="filter-bar">
                                        <span>当前筛选：{activeFilter.label}</span>
                                        <button className="filter-clear" onClick={() => setActiveFilter(null)}>清除筛选</button>
                                    </div>
                                )}
                                <MediaGrid 
                                    libraryId={currentLib.id} 
                                    keyword={searchKeyword} 
                                    filter={activeFilter}
                                    onSelectMedia={setSelectedMedia}
                                    onCountChange={setMediaCount}
                                />
                            </>
                        )}
                        {view === 'watched' && currentLib && !activeFilter && (
                            <>
                                <TopBar libName="已看列表" mediaCount={mediaCount} onSearch={setSearchKeyword} onScan={handleScan} onScanWithMode={handleScanWithMode} />
                                <div className="filter-bar">
                                    <span>当前视图：已看 / 历史记录</span>
                                    <button className="filter-clear" onClick={() => setView('libs')}>返回主墙</button>
                                </div>
                                <MediaGrid 
                                    libraryId={currentLib.id} 
                                    keyword={searchKeyword} 
                                    filter={{ type: 'watched', value: 'true', label: '已看' }}
                                    onSelectMedia={setSelectedMedia}
                                    onCountChange={setMediaCount}
                                />
                            </>
                        )}
                        {view === 'favorite' && currentLib && !activeFilter && (
                            <>
                                <TopBar libName="我的收藏" mediaCount={mediaCount} onSearch={setSearchKeyword} onScan={handleScan} onScanWithMode={handleScanWithMode} />
                                <div className="filter-bar">
                                    <span>当前视图：我的收藏</span>
                                    <button className="filter-clear" onClick={() => setView('libs')}>返回主墙</button>
                                </div>
                                <MediaGrid 
                                    libraryId={currentLib.id} 
                                    keyword={searchKeyword} 
                                    filter={{ type: 'favorite', value: 'true', label: '收藏' }}
                                    onSelectMedia={setSelectedMedia}
                                    onCountChange={setMediaCount}
                                />
                            </>
                        )}
                        {view === 'directory' && currentLib && !activeFilter && (
                            <>
                                <TopBar libName="目录聚合" mediaCount={0} onSearch={() => {}} onScan={() => {}} onScanWithMode={() => {}} />
                                <CategoryGrid type="directory" libraryId={currentLib.id} fetchFn={GetDirectoryStats} onSelect={(val, lbl) => { setActiveFilter({ type: 'directory', value: val, label: lbl }); setView('libs'); }} />
                            </>
                        )}
                        {view === 'actor' && currentLib && !activeFilter && (
                            <>
                                <TopBar libName="演员群" mediaCount={0} onSearch={() => {}} onScan={() => {}} onScanWithMode={() => {}} />
                                <CategoryGrid type="actor" libraryId={currentLib.id} fetchFn={GetActorStats} onSelect={(val, lbl) => { setActiveFilter({ type: 'actor', value: val, label: lbl }); setView('libs'); }} />
                            </>
                        )}
                        {view === 'genre' && currentLib && !activeFilter && (
                            <>
                                <TopBar libName="类别统计" mediaCount={0} onSearch={() => {}} onScan={() => {}} onScanWithMode={() => {}} />
                                <CategoryGrid type="genre" libraryId={currentLib.id} fetchFn={GetGenreStats} onSelect={(val, lbl) => { setActiveFilter({ type: 'genre', value: val, label: lbl }); setView('libs'); }} />
                            </>
                        )}
                        {view === 'series' && currentLib && !activeFilter && (
                            <>
                                <TopBar libName="系列 / 集合" mediaCount={0} onSearch={() => {}} onScan={() => {}} onScanWithMode={() => {}} />
                                <CategoryGrid type="series" libraryId={currentLib.id} fetchFn={GetSeriesStats} onSelect={(val, lbl) => { setActiveFilter({ type: 'series', value: val, label: lbl }); setView('libs'); }} />
                            </>
                        )}
                        {view === 'libs' && !currentLib && (
                            <div style={{ flex: 1, display: 'flex', alignItems: 'center', justifyContent: 'center', color: 'var(--text-dim)' }}>
                                请先新建媒体库
                            </div>
                        )}
                        {view === 'settings' && (
                            <SettingsPage onClose={() => setView('libs')} />
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
