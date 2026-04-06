import React, { useEffect, useState, useRef } from 'react';
import { GetMediaList } from "../../wailsjs/go/main/App";
import MediaCard from './MediaCard';

interface MediaGridProps {
    libraryId: string;
    keyword: string;
    sortField: string;
    sortOrder: 'asc' | 'desc';
    filter?: { type: string; value: string; label: string } | null;
    onSelectMedia: (media: any) => void;
    onCountChange?: (count: number) => void;
    onQuickPlayStatus?: (message: string) => void;
}

const MediaGrid: React.FC<MediaGridProps> = ({ libraryId, keyword, sortField, sortOrder, filter, onSelectMedia, onCountChange, onQuickPlayStatus }) => {
    const [mediaItems, setMediaItems] = useState<any[]>([]);
    const [isLoading, setIsLoading] = useState(true);
    
    // 布局相关的状态
    const containerRef = useRef<HTMLDivElement>(null);
    const [layout, setLayout] = useState({ columns: 4, gap: 30, justify: 'start' });

    // 核心计算逻辑：优先列数，约束间距
    const updateLayout = () => {
        if (!containerRef.current) return;
        const containerWidth = containerRef.current.clientWidth - 60; // 减去 padding (30*2)
        const cardWidth = 178;
        const minGap = 12; // 最小允许间距：决定过快“跨越”到下一列的关键
        const maxGap = 32; // 视觉最大间隙

        // 1. 根据最小间距，计算当前空间最多能放下多少列
        let cols = Math.floor((containerWidth + minGap) / (cardWidth + minGap));
        cols = Math.max(1, cols);

        // 2. 计算在这种列数下，平摊后的实际间距
        let currentGap = cols > 1 ? (containerWidth - cols * cardWidth) / (cols - 1) : 0;
        
        // 3. 结果应用策略：如果间距过大，则改为居中（或者保持 space-between 但限制上限）
        // 这里采用 space-between 以铺满宽度，但因为我们使用了足够小的 minGap，它会尽早触发新增列。
        setLayout({ columns: cols, gap: currentGap, justify: 'space-between' });
    };

    useEffect(() => {
        const observer = new ResizeObserver(() => updateLayout());
        if (containerRef.current) observer.observe(containerRef.current);
        updateLayout(); 
        return () => observer.disconnect();
    }, []);

    useEffect(() => {
        setIsLoading(true);
        let active = true;
        const filterType = filter?.type || "";
        const filterValue = filter?.value || "";

        GetMediaList(libraryId, 1, 100, sortField, sortOrder, keyword, filterType, filterValue)
            .then((res: any) => {
                if (active) {
                    setMediaItems(res.items || []);
                    if (onCountChange) onCountChange(res.total || 0);
                    setIsLoading(false);
                }
            })
            .catch(err => {
                console.error(err);
                if (active) setIsLoading(false);
            });
        return () => { active = false; };
    }, [libraryId, keyword, sortField, sortOrder, filter]);

    return (
        <div 
            ref={containerRef} 
            className="grid-container" 
            style={{ 
                gridTemplateColumns: `repeat(${layout.columns}, 178px)`, 
                columnGap: `${layout.gap}px`,
                justifyContent: layout.justify,
                rowGap: '30px'
            }}
        >
            {mediaItems.map(item => (
                <MediaCard key={item.id} media={item} onClick={() => onSelectMedia(item)} onQuickPlayStatus={onQuickPlayStatus} />
            ))}
            {!isLoading && mediaItems.length === 0 && (
                <div style={{ color: 'var(--text-dim)', padding: '40px', gridColumn: '1 / -1', textAlign: 'center', fontSize: '14px' }}>
                    没有找到符合条件的媒体文件
                </div>
            )}
            {isLoading && mediaItems.length === 0 && (
                <div style={{ color: 'var(--accent)', padding: '40px', gridColumn: '1 / -1', textAlign: 'center', fontSize: '14px' }}>
                    正在加载媒体库...
                </div>
            )}
        </div>
    );
};

export default MediaGrid;
