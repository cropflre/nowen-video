import React from 'react';
import { Play } from 'lucide-react';
import { PlayFile } from "../../wailsjs/go/main/App";

interface MediaCardProps {
    media: any;
    onClick: () => void;
    onQuickPlayStatus?: (message: string) => void;
}

const formatError = (error: unknown) => {
    if (error instanceof Error && error.message) {
        return error.message;
    }
    if (typeof error === 'string') {
        return error;
    }
    return '未知错误';
};

const MediaCard: React.FC<MediaCardProps> = ({ media, onClick, onQuickPlayStatus }) => {
    const coverUrl = media.poster_path
        ? `/local/${media.poster_path}`
        : media.backdrop_path
            ? `/local/${media.backdrop_path}`
            : '';

    const handleQuickPlay = async (event: React.MouseEvent<HTMLButtonElement>) => {
        event.stopPropagation();

        const targetPath = typeof media?.file_path === 'string' ? media.file_path.trim() : '';
        if (!targetPath) {
            onQuickPlayStatus?.('播放失败：当前卡片没有可播放文件');
            return;
        }

        try {
            onQuickPlayStatus?.(`正在启动播放器：${targetPath.split(/[\\/]/).pop()}`);
            await PlayFile(targetPath);
        } catch (error) {
            console.error(error);
            onQuickPlayStatus?.(`播放失败：${formatError(error)}`);
        }
    };

    return (
        <div className="media-card" onClick={onClick}>
            <div className="media-poster-wrapper">
                {coverUrl ? (
                    <img
                        src={coverUrl}
                        className="media-poster-image"
                        alt={media.title}
                        loading="lazy"
                        onError={(event) => {
                            (event.target as HTMLImageElement).src = 'https://via.placeholder.com/178x255?text=No+Poster';
                        }}
                    />
                ) : (
                    <div className="media-poster-empty">
                        No Image
                    </div>
                )}

                <div className="media-card-play-overlay">
                    <button
                        type="button"
                        className="media-card-play-button"
                        aria-label={`播放 ${media.title || '当前媒体'}`}
                        onClick={handleQuickPlay}
                    >
                        <Play size={26} strokeWidth={1.8} className="media-card-play-icon" fill="currentColor" />
                    </button>
                </div>
            </div>

            <div className="media-info">
                <div className="media-title" title={media.title}>
                    {media.title || '未知标题'}
                </div>
                <div className="media-year">
                    {media.year || '未知日期'}
                </div>
            </div>
        </div>
    );
};

export default MediaCard;
