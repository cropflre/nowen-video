import React, { useEffect, useState } from 'react';
import { CreateLibrary, DeleteLibrary, SelectDirectory, UpdateLibrary } from "../../wailsjs/go/main/App";
import {
    buildLibraryPayload,
    DEFAULT_LIBRARY_SUBTITLE_FIELD,
    DEFAULT_LIBRARY_TITLE_FIELD,
    DEFAULT_LIBRARY_VIEW_MODE,
    getLibraryConfig,
} from '../utils/library';

interface LibraryFormModalProps {
    mode: 'create' | 'edit';
    library?: any;
    onClose: () => void;
    onSaved: () => void;
    onDeleted?: () => void;
}

const emptyConfig = {
    folderPaths: [] as string[],
    viewMode: DEFAULT_LIBRARY_VIEW_MODE,
    titleField: DEFAULT_LIBRARY_TITLE_FIELD,
    subtitleField: DEFAULT_LIBRARY_SUBTITLE_FIELD,
};

const LibraryFormModal: React.FC<LibraryFormModalProps> = ({
    mode,
    library,
    onClose,
    onSaved,
    onDeleted,
}) => {
    const [name, setName] = useState('');
    const [folderPaths, setFolderPaths] = useState<string[]>([]);
    const [manualPath, setManualPath] = useState('');
    const [viewMode, setViewMode] = useState(DEFAULT_LIBRARY_VIEW_MODE);
    const [titleField, setTitleField] = useState(DEFAULT_LIBRARY_TITLE_FIELD);
    const [subtitleField, setSubtitleField] = useState(DEFAULT_LIBRARY_SUBTITLE_FIELD);
    const [msg, setMsg] = useState('');

    const resetForm = () => {
        const config = library ? getLibraryConfig(library) : emptyConfig;
        setName(library?.name || '');
        setFolderPaths(config.folderPaths);
        setViewMode(config.viewMode);
        setTitleField(config.titleField);
        setSubtitleField(config.subtitleField);
        setManualPath('');
        setMsg('');
    };

    useEffect(() => {
        resetForm();
    }, [library, mode]);

    const pushPath = (path: string) => {
        const trimmed = path.trim();
        if (!trimmed) {
            return;
        }
        setFolderPaths((prev) => (prev.includes(trimmed) ? prev : [...prev, trimmed]));
        setManualPath('');
    };

    const handleSelectDir = async () => {
        try {
            const dir = await SelectDirectory();
            if (dir) {
                pushPath(dir);
            }
        } catch (error) {
            console.error(error);
        }
    };

    const handleSave = async () => {
        if (!name.trim()) {
            setMsg('媒体库名称不能为空');
            return;
        }
        if (folderPaths.length === 0) {
            setMsg('请至少保留一个文件夹路径');
            return;
        }

        const payload = buildLibraryPayload(library || {}, {
            name: name.trim(),
            folderPaths,
            viewMode,
            titleField,
            subtitleField,
        });

        try {
            if (mode === 'create') {
                await CreateLibrary({
                    ...payload,
                    type: library?.type || 'movie',
                    metadata_mode: library?.metadata_mode || 'online_preferred',
                } as any);
            } else {
                await UpdateLibrary(payload as any);
            }
            onSaved();
        } catch (error: any) {
            setMsg(`${mode === 'create' ? '创建' : '保存'}失败：${error}`);
        }
    };

    const handleDelete = async () => {
        if (!library || !onDeleted) {
            return;
        }
        if (!window.confirm('确定要删除此媒体库及其媒体记录吗？此操作不可撤销。')) {
            return;
        }

        try {
            await DeleteLibrary(library.id);
            onDeleted();
        } catch (error: any) {
            setMsg(`删除失败：${error}`);
        }
    };

    const isCreateMode = mode === 'create';

    return (
        <div className="modal-overlay" onClick={onClose}>
            <div className="library-edit-modal" onClick={(event) => event.stopPropagation()}>
                <div className="library-edit-header">
                    <span>{isCreateMode ? '新建媒体库' : '编辑媒体库'}</span>
                    <button type="button" className="library-edit-close" onClick={onClose}>×</button>
                </div>

                <div className="library-edit-body">
                    <div className="library-edit-field">
                        <label>媒体库名</label>
                        <input
                            className="library-edit-input"
                            value={name}
                            onChange={(event) => setName(event.target.value)}
                        />
                    </div>

                    <div className="library-edit-grid">
                        <div className="library-edit-field">
                            <label>视图</label>
                            <select
                                className="library-edit-select"
                                value={viewMode}
                                onChange={(event) => setViewMode(event.target.value)}
                            >
                                <option value="poster">海报图</option>
                                <option value="compact">紧凑图</option>
                            </select>
                        </div>

                        <div className="library-edit-field">
                            <label>标题</label>
                            <select
                                className="library-edit-select"
                                value={titleField}
                                onChange={(event) => setTitleField(event.target.value)}
                            >
                                <option value="title">标题</option>
                                <option value="code">视频编码</option>
                                <option value="orig_title">原标题</option>
                            </select>
                        </div>

                        <div className="library-edit-field">
                            <label>副标题</label>
                            <select
                                className="library-edit-select"
                                value={subtitleField}
                                onChange={(event) => setSubtitleField(event.target.value)}
                            >
                                <option value="year">年份</option>
                                <option value="release_date">发行日期</option>
                                <option value="none">无</option>
                            </select>
                        </div>
                    </div>

                    <div className="library-edit-field">
                        <label>文件夹路径</label>
                        <div className="library-edit-path-actions">
                            <button type="button" className="library-edit-small-btn" onClick={handleSelectDir}>+ 文件夹</button>
                            <button type="button" className="library-edit-small-btn secondary" onClick={() => pushPath(manualPath)}>手动添加</button>
                        </div>
                        <input
                            className="library-edit-input"
                            value={manualPath}
                            onChange={(event) => setManualPath(event.target.value)}
                            placeholder="输入路径后点击手动添加"
                        />
                    </div>

                    <div className="library-path-list">
                        <div className="library-path-list-header">
                            <span className="index">序号</span>
                            <span className="path">路径</span>
                            <span className="actions">操作</span>
                        </div>

                        {folderPaths.length > 0 ? folderPaths.map((path, index) => (
                            <div key={path} className="library-path-row">
                                <span className="index">{index + 1}</span>
                                <span className="path" title={path}>{path}</span>
                                <span className="actions">
                                    <button
                                        type="button"
                                        className="library-path-action danger"
                                        onClick={() => setFolderPaths((prev) => prev.filter((item) => item !== path))}
                                    >
                                        删除
                                    </button>
                                </span>
                            </div>
                        )) : (
                            <div className="library-path-empty">尚未添加文件夹路径</div>
                        )}
                    </div>
                </div>

                <div className="library-edit-footer">
                    {msg && <span className="library-edit-msg">{msg}</span>}
                    <div className="library-edit-footer-actions">
                        {!isCreateMode && (
                            <button type="button" className="library-edit-footer-btn danger" onClick={handleDelete}>删除</button>
                        )}
                        <button type="button" className="library-edit-footer-btn" onClick={resetForm}>重置</button>
                        <button type="button" className="library-edit-footer-btn" onClick={onClose}>取消</button>
                        <button type="button" className="library-edit-footer-btn primary" onClick={handleSave}>
                            {isCreateMode ? '保存' : '保存'}
                        </button>
                    </div>
                </div>
            </div>
        </div>
    );
};

export default LibraryFormModal;
