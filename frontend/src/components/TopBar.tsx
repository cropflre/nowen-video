import React, { useEffect, useRef, useState } from 'react';
import { Search } from 'lucide-react';

type MenuType = 'scan' | 'sort' | null;

interface TopBarProps {
    libName: string;
    mediaCount: number;
    searchValue: string;
    onSearch: (keyword: string) => void;
    onScanWithMode: (mode: string) => void;
    onEditLibrary?: () => void;
    onRandomPlay?: () => void;
    onSortSelect?: (field: string) => void;
    sortField?: string;
    sortOrder?: 'asc' | 'desc';
    showLibraryControls?: boolean;
}

const sortOptions = [
    { field: 'created_at', label: '加入日期' },
    { field: 'release_date', label: '发行日期' },
    { field: 'video_codec', label: '视频编码' },
    { field: 'last_watched', label: '最近观看' },
];

const scanOptions = [
    { mode: 'overwrite', label: '覆盖刷新' },
    { mode: 'delete_update', label: '删改刷新' },
    { mode: 'incremental', label: '新增刷新' },
];

const getSortLabel = (field: string) => {
    switch (field) {
        case 'release_date':
            return '发行日期';
        case 'video_codec':
            return '视频编码';
        case 'last_watched':
            return '最近观看';
        case 'created_at':
        default:
            return '加入日期';
    }
};

const TopBar: React.FC<TopBarProps> = ({
    libName,
    mediaCount,
    searchValue,
    onSearch,
    onScanWithMode,
    onEditLibrary,
    onRandomPlay,
    onSortSelect,
    sortField = 'created_at',
    sortOrder = 'desc',
    showLibraryControls = true,
}) => {
    const [openMenu, setOpenMenu] = useState<MenuType>(null);
    const [confirmScanMode, setConfirmScanMode] = useState<'overwrite' | null>(null);
    const menuRootRef = useRef<HTMLDivElement | null>(null);

    useEffect(() => {
        const handlePointerDown = (event: MouseEvent) => {
            if (!menuRootRef.current?.contains(event.target as Node)) {
                setOpenMenu(null);
            }
        };

        const handleEscape = (event: KeyboardEvent) => {
            if (event.key === 'Escape') {
                setOpenMenu(null);
                setConfirmScanMode(null);
            }
        };

        document.addEventListener('mousedown', handlePointerDown);
        document.addEventListener('keydown', handleEscape);
        return () => {
            document.removeEventListener('mousedown', handlePointerDown);
            document.removeEventListener('keydown', handleEscape);
        };
    }, []);

    const handleScanModeClick = (mode: string) => {
        setOpenMenu(null);
        if (mode === 'overwrite') {
            setConfirmScanMode('overwrite');
            return;
        }
        onScanWithMode(mode);
    };

    const sortArrow = sortOrder === 'desc' ? '↓' : '↑';
    const sortLabel = `按${getSortLabel(sortField)}排序`;

    return (
        <>
            <div className="topbar" ref={menuRootRef}>
                <span className="topbar-hamburger">☰</span>
                <span className="topbar-lib-name">{libName || '媒体库'}</span>
                <span className="topbar-count">{mediaCount || 0} 个项目</span>

                {showLibraryControls && (
                    <>
                        <button className="topbar-btn" type="button" onClick={onRandomPlay} disabled={!onRandomPlay}>
                            <span className="topbar-btn-icon">⤮</span>
                            <span>随机播放</span>
                        </button>

                        <div className="topbar-menu-anchor">
                            <button
                                type="button"
                                className={`topbar-btn ${openMenu === 'sort' ? 'active' : ''}`}
                                onClick={() => setOpenMenu((prev) => (prev === 'sort' ? null : 'sort'))}
                            >
                                <span className="topbar-btn-icon">{sortArrow}</span>
                                <span>{sortLabel}</span>
                                <span className="topbar-btn-caret">▾</span>
                            </button>

                            {openMenu === 'sort' && (
                                <div className="topbar-dropdown-menu">
                                    {sortOptions.map((option) => {
                                        const isActive = sortField === option.field;
                                        return (
                                            <button
                                                key={option.field}
                                                type="button"
                                                className={`topbar-dropdown-item ${isActive ? 'active' : ''}`}
                                                onClick={() => {
                                                    onSortSelect?.(option.field);
                                                    setOpenMenu(null);
                                                }}
                                            >
                                                <span>{option.label}</span>
                                                {isActive && <span>{sortArrow}</span>}
                                            </button>
                                        );
                                    })}
                                </div>
                            )}
                        </div>

                        <div className="topbar-menu-anchor">
                            <button
                                type="button"
                                className={`topbar-btn icon-only ${openMenu === 'scan' ? 'active' : ''}`}
                                onClick={() => setOpenMenu((prev) => (prev === 'scan' ? null : 'scan'))}
                                title="刷新"
                            >
                                <span className="topbar-btn-icon">↻</span>
                            </button>

                            {openMenu === 'scan' && (
                                <div className="topbar-dropdown-menu narrow">
                                    {scanOptions.map((option) => (
                                        <button
                                            key={option.mode}
                                            type="button"
                                            className="topbar-dropdown-item"
                                            onClick={() => handleScanModeClick(option.mode)}
                                        >
                                            <span>{option.label}</span>
                                        </button>
                                    ))}
                                </div>
                            )}
                        </div>

                        <button
                            type="button"
                            className="topbar-btn icon-only"
                            title="编辑当前媒体库"
                            onClick={() => onEditLibrary?.()}
                            disabled={!onEditLibrary}
                        >
                            <span className="topbar-btn-icon">⋯</span>
                        </button>
                    </>
                )}

                <div className="topbar-search-container">
                    <div className="topbar-search-shell">
                        <span className="topbar-search-icon-wrap">
                            <Search size={14} strokeWidth={2.1} className="topbar-search-icon" />
                        </span>
                        <input
                            type="text"
                            className="topbar-search"
                            placeholder="搜索媒体、演员、标签..."
                            value={searchValue}
                            onChange={(event) => onSearch(event.target.value)}
                        />
                    </div>
                </div>
            </div>

            {confirmScanMode === 'overwrite' && (
                <div className="modal-overlay" onClick={() => setConfirmScanMode(null)}>
                    <div className="confirm-modal" onClick={(event) => event.stopPropagation()}>
                        <div className="confirm-modal-header">
                            <span>提示</span>
                            <button type="button" className="confirm-modal-close" onClick={() => setConfirmScanMode(null)}>×</button>
                        </div>

                        <div className="confirm-modal-body">
                            <div className="confirm-modal-icon">?</div>
                            <div className="confirm-modal-text">
                                你确定要覆盖刷新吗？这样会清空当前媒体库。
                            </div>
                        </div>

                        <div className="confirm-modal-actions">
                            <button type="button" className="confirm-modal-btn ghost" onClick={() => setConfirmScanMode(null)}>取消</button>
                            <button
                                type="button"
                                className="confirm-modal-btn primary"
                                onClick={() => {
                                    onScanWithMode('overwrite');
                                    setConfirmScanMode(null);
                                }}
                            >
                                确定
                            </button>
                        </div>
                    </div>
                </div>
            )}
        </>
    );
};

export default TopBar;
