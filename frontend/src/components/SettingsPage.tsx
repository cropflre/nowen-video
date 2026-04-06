import React, { useState, useEffect } from 'react';
import { GetDesktopSettings, UpdateDesktopSettings, RestartApp } from "../../wailsjs/go/main/App";

interface SettingsPageProps {
    onClose: () => void;
}

const SettingsPage: React.FC<SettingsPageProps> = ({ onClose }) => {
    const [settings, setSettings] = useState<any>(null);
    const [activeTab, setActiveTab] = useState<'appearance' | 'scan' | 'emby' | 'about'>('appearance');
    const [msg, setMsg] = useState('');

    useEffect(() => {
        GetDesktopSettings().then((res: any) => {
            if (res) setSettings(res);
        }).catch(console.error);
    }, []);

    const handleSave = async () => {
        try {
            await UpdateDesktopSettings(settings);
            setMsg("设置已保存，部分项目需重启生效。");
            setTimeout(() => setMsg(''), 3000);
        } catch (e: any) {
            setMsg("保存失败: " + e);
        }
    };

    const handleRestart = () => {
        if (window.confirm("确定要重启软件吗？")) {
            RestartApp();
        }
    };

    if (!settings) return <div style={{ padding: '20px', color: 'var(--text-dim)' }}>加载中...</div>;

    const renderAppearance = () => (
        <>
            <div className="settings-section">
                <div className="settings-section-title">外观</div>

                <div style={{ display: 'flex', gap: '40px' }}>
                    <div style={{ flex: 1 }}>
                        <div className="settings-row">
                            <div className="settings-label">程序皮肤颜色</div>
                            <div className="settings-control">
                                <select className="settings-select" value={settings.theme} onChange={e => setSettings({ ...settings, theme: e.target.value })}>
                                    <option value="dark">黑色</option>
                                    <option value="light">白色</option>
                                </select>
                            </div>
                        </div>

                        <div className="settings-row">
                            <div className="settings-label">封面圆角度</div>
                            <div className="settings-control">
                                <select className="settings-select" value={settings.poster_radius} onChange={e => setSettings({ ...settings, poster_radius: parseInt(e.target.value) || 0 })}>
                                    <option value="0">0</option>
                                    <option value="4">4</option>
                                    <option value="10">10</option>
                                </select>
                            </div>
                        </div>

                        <div className="settings-row">
                            <div className="settings-label">背景模糊度</div>
                            <div className="settings-control">
                                <select className="settings-select" value={settings.backdrop_blur} onChange={e => setSettings({ ...settings, backdrop_blur: parseInt(e.target.value) || 0 })}>
                                    <option value="0">0</option>
                                    <option value="5">5</option>
                                    <option value="10">10</option>
                                </select>
                            </div>
                        </div>

                        <div className="settings-row" style={{ marginTop: '16px' }}>
                            <label className="settings-checkbox-row">
                                <input type="checkbox" checked={settings.min_window_width > 0} onChange={e => setSettings({ ...settings, min_window_width: e.target.checked ? 740 : 0 })} />
                                <span className="settings-checkbox-label">窗口最小宽度</span>
                            </label>
                            {settings.min_window_width > 0 && (
                                <div className="settings-control" style={{ width: '60px', marginLeft: '10px' }}>
                                    <input className="settings-input" type="number" value={settings.min_window_width || 740} onChange={e => setSettings({ ...settings, min_window_width: parseInt(e.target.value) || 740 })} />
                                </div>
                            )}
                        </div>
                    </div>
                </div>
            </div>

            <div className="settings-section">
                <div className="settings-section-title">播放</div>
                <div style={{ display: 'flex', alignItems: 'center', gap: '16px', marginBottom: '16px' }}>
                    <label className="settings-checkbox-row" style={{ marginBottom: 0 }}>
                        <input type="checkbox" checked={settings.use_external_player} onChange={e => setSettings({ ...settings, use_external_player: e.target.checked })} />
                        <span className="settings-checkbox-label" style={{ fontWeight: 600 }}>调用本地播放器</span>
                    </label>
                    <div style={{ display: 'flex', alignItems: 'center', gap: '8px', flex: 1, maxWidth: '300px' }}>
                        <span style={{ fontSize: '12px', color: 'var(--text-secondary)' }}>路径:</span>
                        <input className="settings-input" type="text" value={settings.player_path || ''} onChange={e => setSettings({ ...settings, player_path: e.target.value })} disabled={!settings.use_external_player} />
                    </div>
                </div>
            </div>

            <div className="settings-section">
                <div className="settings-section-title">快捷</div>

                <div className="settings-row" style={{ marginBottom: '20px' }}>
                    <div className="settings-label">热键注册失败</div>
                    <div className="settings-control">
                        <select className="settings-select" value={settings.hotkey || 'F1'} onChange={e => setSettings({ ...settings, hotkey: e.target.value })}>
                            <option value="F1">F1</option>
                            <option value="F2">F2</option>
                        </select>
                    </div>
                </div>

                <div className="settings-checkbox-group" style={{ gap: '30px' }}>
                    <div style={{ display: 'flex', flexDirection: 'column', gap: '12px' }}>
                        <label className="settings-checkbox-row">
                            <input type="checkbox" checked={settings.min_to_tray} onChange={e => setSettings({ ...settings, min_to_tray: e.target.checked })} />
                            <span className="settings-checkbox-label">最小化到托盘</span>
                        </label>
                        <label className="settings-checkbox-row">
                            <input type="checkbox" checked={settings.show_prompt} onChange={e => setSettings({ ...settings, show_prompt: e.target.checked })} />
                            <span className="settings-checkbox-label">打开提示</span>
                        </label>
                        <label className="settings-checkbox-row">
                            <input type="checkbox" checked={settings.start_with_os} onChange={e => setSettings({ ...settings, start_with_os: e.target.checked })} />
                            <span className="settings-checkbox-label">开机启动</span>
                        </label>
                    </div>
                    <div style={{ display: 'flex', flexDirection: 'column', gap: '12px' }}>
                        <label className="settings-checkbox-row">
                            <input type="checkbox" checked={settings.max_no_taskbar} onChange={e => setSettings({ ...settings, max_no_taskbar: e.target.checked })} />
                            <span className="settings-checkbox-label">最大化不遮任务栏</span>
                        </label>
                    </div>
                </div>

                <div className="settings-footer" style={{ marginTop: '20px', borderTop: 'none', paddingTop: 0 }}>
                    <button className="settings-btn settings-btn-primary" onClick={handleRestart}>软件重启</button>
                    <button className="settings-btn" onClick={handleSave}>立即保存设置</button>
                    {msg && <span style={{ fontSize: '11px', color: 'var(--accent)' }}>{msg}</span>}
                </div>
            </div>
        </>
    );

    const renderPlaceholder = (title: string) => (
        <div className="settings-section">
            <div className="settings-section-title">{title} (UI Mock)</div>
            <div style={{ color: 'var(--text-dim)', fontSize: '13px' }}>该页面的重组不在本轮目标中。</div>
        </div>
    );

    return (
        <div className="settings-container">
            <div className="settings-sidebar">
                <div className="settings-sidebar-header">设置</div>
                <div className={`settings-nav-item ${activeTab === 'appearance' ? 'active' : ''}`} onClick={() => setActiveTab('appearance')}>常规设置</div>
                <div className={`settings-nav-item ${activeTab === 'scan' ? 'active' : ''}`} onClick={() => setActiveTab('scan')}>扫描</div>
                <div className={`settings-nav-item ${activeTab === 'emby' ? 'active' : ''}`} onClick={() => setActiveTab('emby')}>emby</div>
                <div className={`settings-nav-item ${activeTab === 'about' ? 'active' : ''}`} onClick={() => setActiveTab('about')}>关于</div>
                <div className="sidebar-spacer"></div>
                <div className="settings-return-btn" onClick={onClose}>返回</div>
            </div>

            <div className="settings-main">
                {activeTab === 'appearance' && renderAppearance()}
                {activeTab === 'scan' && renderPlaceholder('扫描设置')}
                {activeTab === 'emby' && renderPlaceholder('Emby 设置')}
                {activeTab === 'about' && renderPlaceholder('关于')}
            </div>
        </div>
    );
};

export default SettingsPage;
