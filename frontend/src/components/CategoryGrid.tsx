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
        let active = true;
        setLoading(true);

        fetchFn(libraryId)
            .then((res) => {
                if (!active) {
                    return;
                }
                const nextItems = Array.isArray(res) ? [...res] : [];
                nextItems.sort((left, right) => {
                    if (right.count !== left.count) {
                        return right.count - left.count;
                    }
                    return left.name.localeCompare(right.name, 'zh-CN');
                });
                setItems(nextItems);
                setLoading(false);
            })
            .catch((error) => {
                console.error(error);
                if (!active) {
                    return;
                }
                setItems([]);
                setLoading(false);
            });

        return () => {
            active = false;
        };
    }, [fetchFn, libraryId, type]);

    if (loading) {
        return (
            <div className="stats-grid-loading">
                正在加载...
            </div>
        );
    }

    return (
        <div className={`stats-grid-container stats-grid-container-${type}`}>
            <div className={`stats-grid stats-grid-${type}`}>
                {items.map((item) => (
                    <button
                        key={item.filter_value || item.name}
                        type="button"
                        className={`stats-card stats-card-${type}`}
                        onClick={() => onSelect(item.filter_value, item.name)}
                    >
                        <span className="stats-card-name" title={item.name}>
                            {item.name}
                        </span>
                        <span className="stats-card-count">({item.count})</span>
                    </button>
                ))}
            </div>

            {items.length === 0 && (
                <div className="stats-grid-empty">
                    暂无数据
                </div>
            )}
        </div>
    );
};

export default CategoryGrid;
