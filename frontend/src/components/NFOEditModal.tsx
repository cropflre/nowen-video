import React, { useEffect, useState } from 'react';

interface NFOEditModalProps {
    data: any;
    loading: boolean;
    saving: boolean;
    onClose: () => void;
    onSave: (draft: any) => Promise<void>;
}

const emptyForm = {
    nfo_path: '',
    title: '',
    code: '',
    release_date: '',
    director: '',
    series: '',
    publisher: '',
    maker: '',
    genres: '',
    actors: '',
    plot: '',
    runtime: '',
    file_size: '',
    resolution: '',
    video_codec: '',
    rating: '',
};

const NFOEditModal: React.FC<NFOEditModalProps> = ({
    data,
    loading,
    saving,
    onClose,
    onSave,
}) => {
    const [form, setForm] = useState<any>(emptyForm);

    useEffect(() => {
        setForm({
            ...emptyForm,
            ...(data || {}),
        });
    }, [data]);

    useEffect(() => {
        const handleEscape = (event: KeyboardEvent) => {
            if (event.key === 'Escape' && !saving) {
                onClose();
            }
        };

        document.addEventListener('keydown', handleEscape);
        return () => document.removeEventListener('keydown', handleEscape);
    }, [onClose, saving]);

    const updateField = (key: string, value: string) => {
        setForm((prev: any) => ({
            ...prev,
            [key]: value,
        }));
    };

    return (
        <div className="modal-overlay" onClick={() => !saving && onClose()}>
            <div className="nfo-edit-modal" onClick={(event) => event.stopPropagation()}>
                <div className="nfo-edit-header">
                    <div className="nfo-edit-header-main">
                        <div className="nfo-edit-title">ALEX 编辑NFO</div>
                        {form.nfo_path && (
                            <div className="nfo-edit-path" title={form.nfo_path}>
                                {form.nfo_path}
                            </div>
                        )}
                    </div>

                    <button
                        type="button"
                        className="nfo-edit-close"
                        onClick={onClose}
                        disabled={saving}
                    >
                        ×
                    </button>
                </div>

                <div className="nfo-edit-body">
                    {loading ? (
                        <div className="nfo-edit-loading">正在读取 NFO...</div>
                    ) : (
                        <>
                            <div className="nfo-edit-row">
                                <label className="nfo-edit-label">片名</label>
                                <input
                                    className="nfo-edit-input"
                                    value={form.title}
                                    onChange={(event) => updateField('title', event.target.value)}
                                />
                            </div>

                            <div className="nfo-edit-grid three">
                                <div className="nfo-edit-field">
                                    <label className="nfo-edit-label">编号</label>
                                    <input
                                        className="nfo-edit-input"
                                        value={form.code}
                                        onChange={(event) => updateField('code', event.target.value)}
                                    />
                                </div>
                                <div className="nfo-edit-field">
                                    <label className="nfo-edit-label">日期</label>
                                    <input
                                        className="nfo-edit-input"
                                        value={form.release_date}
                                        onChange={(event) => updateField('release_date', event.target.value)}
                                        placeholder="YYYY-MM-DD"
                                    />
                                </div>
                                <div className="nfo-edit-field">
                                    <label className="nfo-edit-label">导演</label>
                                    <input
                                        className="nfo-edit-input"
                                        value={form.director}
                                        onChange={(event) => updateField('director', event.target.value)}
                                    />
                                </div>
                            </div>

                            <div className="nfo-edit-grid three">
                                <div className="nfo-edit-field">
                                    <label className="nfo-edit-label">系列</label>
                                    <input
                                        className="nfo-edit-input"
                                        value={form.series}
                                        onChange={(event) => updateField('series', event.target.value)}
                                    />
                                </div>
                                <div className="nfo-edit-field">
                                    <label className="nfo-edit-label">发行</label>
                                    <input
                                        className="nfo-edit-input"
                                        value={form.publisher}
                                        onChange={(event) => updateField('publisher', event.target.value)}
                                    />
                                </div>
                                <div className="nfo-edit-field">
                                    <label className="nfo-edit-label">制作</label>
                                    <input
                                        className="nfo-edit-input"
                                        value={form.maker}
                                        onChange={(event) => updateField('maker', event.target.value)}
                                    />
                                </div>
                            </div>

                            <div className="nfo-edit-row">
                                <label className="nfo-edit-label">类别</label>
                                <input
                                    className="nfo-edit-input"
                                    value={form.genres}
                                    onChange={(event) => updateField('genres', event.target.value)}
                                    placeholder="多个值可用 / 或 , 分隔"
                                />
                            </div>

                            <div className="nfo-edit-row">
                                <label className="nfo-edit-label">演员</label>
                                <input
                                    className="nfo-edit-input"
                                    value={form.actors}
                                    onChange={(event) => updateField('actors', event.target.value)}
                                    placeholder="多个值可用 / 或 , 分隔"
                                />
                            </div>

                            <div className="nfo-edit-row textarea">
                                <label className="nfo-edit-label">简介</label>
                                <textarea
                                    className="nfo-edit-textarea"
                                    value={form.plot}
                                    onChange={(event) => updateField('plot', event.target.value)}
                                />
                            </div>

                            <div className="nfo-edit-grid metrics">
                                <div className="nfo-edit-field">
                                    <label className="nfo-edit-label">时长</label>
                                    <input
                                        className="nfo-edit-input compact"
                                        value={form.runtime}
                                        onChange={(event) => updateField('runtime', event.target.value)}
                                    />
                                </div>
                                <div className="nfo-edit-field">
                                    <label className="nfo-edit-label">大小</label>
                                    <input className="nfo-edit-input compact readonly" value={form.file_size} readOnly />
                                </div>
                                <div className="nfo-edit-field">
                                    <label className="nfo-edit-label">分辨率</label>
                                    <input className="nfo-edit-input compact readonly" value={form.resolution} readOnly />
                                </div>
                                <div className="nfo-edit-field">
                                    <label className="nfo-edit-label">编码</label>
                                    <input className="nfo-edit-input compact readonly" value={form.video_codec} readOnly />
                                </div>
                                <div className="nfo-edit-field">
                                    <label className="nfo-edit-label">评分</label>
                                    <input
                                        className="nfo-edit-input compact"
                                        value={form.rating}
                                        onChange={(event) => updateField('rating', event.target.value)}
                                    />
                                </div>
                            </div>
                        </>
                    )}
                </div>

                <div className="nfo-edit-footer">
                    <button type="button" className="nfo-edit-action ghost" onClick={onClose} disabled={saving}>
                        取消
                    </button>
                    <button
                        type="button"
                        className="nfo-edit-action primary"
                        onClick={() => onSave(form)}
                        disabled={loading || saving}
                    >
                        {saving ? '保存中...' : '保存'}
                    </button>
                </div>
            </div>
        </div>
    );
};

export default NFOEditModal;
