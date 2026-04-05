import React, { useEffect, useState } from 'react';

interface StatsItem {
    name: string;
    count: number;
    image: string;
    filter_value: string;
}

interface CategoryGridProps {
    type: 'directory' | 'actor' | 'genre' | 'series';
    libraryId: string;
    onSelect: (value: string, label: string) => void;
    fetchFn: (libId: string) => Promise<StatsItem[]>;
}

const CategoryGrid: React.FC<CategoryGridProps> = ({ type, libraryId, onSelect, fetchFn }) => {
    const [items, setItems] = useState<StatsItem[]>([]);
    const [loading, setLoading] = useState(true);

    useEffect(() => {
        setLoading(true);
        fetchFn(libraryId)
            .then(res => {
                setItems(res || []);
                setLoading(false);
            })
            .catch(err => {
                console.error(err);
                setItems([]);
                setLoading(false);
            });
    }, [libraryId, type]);

    if (loading) {
        return <div style={{ color: 'var(--accent)', padding: '40px', textAlign: 'center', fontSize: '14px' }}>加载中...</div>;
    }

    const getLabelPrefix = () => {
        switch (type) {
            case 'directory': return '目录';
            case 'actor': return '演员群';
            case 'genre': return '类别';
            case 'series': return '系列';
            default: return '';
        }
    };

    return (
        <div className="stats-grid-container">
            <div className="stats-grid">
                {items.map((item, idx) => (
                    <div 
                        key={idx} 
                        className="stats-card"
                        onClick={() => onSelect(item.filter_value, `${getLabelPrefix()}: ${item.name}`)}
                    >
                        <div className="stats-card-name" title={item.name}>
                            {item.name}
                            <span style={{color: 'var(--text-dim)', marginLeft: '4px'}}>
                                ({item.count})
                            </span>
                        </div>
                    </div>
                ))}
            </div>
            {items.length === 0 && (
                <div style={{ color: 'var(--text-dim)', padding: '40px', textAlign: 'center', fontSize: '14px' }}>
                    暂无数据
                </div>
            )}
        </div>
    );
};

export default CategoryGrid;
