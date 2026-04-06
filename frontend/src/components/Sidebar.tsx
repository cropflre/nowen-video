import React, { useState } from 'react';
import { ChevronRight } from 'lucide-react';
import { formatLibraryPathLabel, getLibraryConfig } from '../utils/library';

interface SidebarProps {
    libraries: any[];
    currentLib: any;
    currentView: string;
    onSelectLib: (lib: any) => void;
    onOpenSettings: () => void;
    onSelectView: (view: 'libs' | 'directory' | 'actor' | 'genre' | 'series' | 'watched' | 'favorite') => void;
    onAddLib: () => void;
    onEditLib: (lib: any) => void;
}

const Sidebar: React.FC<SidebarProps> = ({
    libraries,
    currentLib,
    currentView,
    onSelectLib,
    onOpenSettings,
    onSelectView,
    onAddLib,
    onEditLib,
}) => {
    const [expandedLibId, setExpandedLibId] = useState<string | null>(null);

    const toggleLibraryFolders = (libraryId: string) => {
        setExpandedLibId((prev) => (prev === libraryId ? null : libraryId));
    };

    return (
        <div className="sidebar">
            <div className="sidebar-logo">
                <i>▶</i>
                <span>ALEX</span>
            </div>

            <div className="sidebar-nav-item sidebar-create-lib" onClick={onAddLib}>
                <span>新建媒体库</span>
                <span className="sidebar-create-lib-icon">•</span>
            </div>

            <div className="sidebar-section">
                <div className="sidebar-section-title">
                    <span>我的媒体</span>
                    <span className="sidebar-section-arrow">▾</span>
                </div>

                {libraries.map((lib) => {
                    const isActive = currentLib?.id === lib.id && currentView === 'libs';
                    const isExpanded = expandedLibId === lib.id;
                    const { folderPaths } = getLibraryConfig(lib);

                    return (
                        <div key={lib.id} className={`sidebar-lib-group ${isExpanded ? 'expanded' : ''}`}>
                            <div className={`sidebar-lib-row ${isActive ? 'active' : ''}`}>
                                <button
                                    type="button"
                                    className={`sidebar-lib-toggle ${isExpanded ? 'expanded' : ''}`}
                                    onClick={() => toggleLibraryFolders(lib.id)}
                                    aria-label={isExpanded ? '折叠文件夹' : '展开文件夹'}
                                >
                                    <ChevronRight size={14} className="sidebar-lib-toggle-icon" />
                                </button>

                                <button
                                    type="button"
                                    className="sidebar-lib-main"
                                    onClick={() => onSelectLib(lib)}
                                >
                                    <span className="sidebar-lib-name" title={lib.name}>
                                        {lib.name}
                                    </span>
                                    <span className="sidebar-count">{lib.media_count?.toLocaleString() || 0}</span>
                                </button>

                                <button
                                    type="button"
                                    className="sidebar-lib-edit"
                                    title="编辑媒体库"
                                    onClick={(event) => {
                                        event.stopPropagation();
                                        onEditLib(lib);
                                    }}
                                >
                                    ⋯
                                </button>
                            </div>

                            {isExpanded && (
                                <div className="sidebar-lib-paths">
                                    {folderPaths.length > 0 ? (
                                        folderPaths.map((path) => (
                                            <div key={path} className="sidebar-lib-path" title={path}>
                                                <span className="sidebar-lib-path-label">
                                                    {formatLibraryPathLabel(path, folderPaths)}
                                                </span>
                                            </div>
                                        ))
                                    ) : (
                                        <div className="sidebar-lib-path empty">未配置文件夹</div>
                                    )}
                                </div>
                            )}
                        </div>
                    );
                })}
            </div>

            <div className="sidebar-section">
                <div className="sidebar-section-title">
                    <span>信息统计</span>
                    <span className="sidebar-section-arrow">▾</span>
                </div>
                <div className={`sidebar-nav-item ${currentView === 'watched' ? 'active' : ''}`} onClick={() => onSelectView('watched')}>
                    已看
                </div>
                <div className={`sidebar-nav-item ${currentView === 'favorite' ? 'active' : ''}`} onClick={() => onSelectView('favorite')}>
                    收藏
                </div>
                <div className={`sidebar-nav-item ${currentView === 'actor' ? 'active' : ''}`} onClick={() => onSelectView('actor')}>
                    演员
                </div>
                <div className={`sidebar-nav-item ${currentView === 'genre' ? 'active' : ''}`} onClick={() => onSelectView('genre')}>
                    类别
                </div>
            </div>

            <div className="sidebar-section">
                <div className="sidebar-section-title">
                    <span>服务选项</span>
                    <span className="sidebar-section-arrow">▾</span>
                </div>
            </div>

            <div className="sidebar-spacer" />

            <div className="sidebar-bottom">
                <div
                    className={`sidebar-nav-item ${currentView === 'settings' ? 'active' : ''}`}
                    style={{
                        background: currentView === 'settings' ? 'var(--accent)' : 'transparent',
                        color: currentView === 'settings' ? '#fff' : 'var(--text-secondary)',
                    }}
                    onClick={onOpenSettings}
                >
                    设置
                </div>
            </div>
        </div>
    );
};

export default Sidebar;
