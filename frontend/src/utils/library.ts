export const DEFAULT_LIBRARY_VIEW_MODE = 'poster';
export const DEFAULT_LIBRARY_TITLE_FIELD = 'title';
export const DEFAULT_LIBRARY_SUBTITLE_FIELD = 'year';

const normalizeFolderPaths = (paths: string[]) => {
    const seen = new Set<string>();
    return paths
        .map((path) => (typeof path === 'string' ? path.trim() : ''))
        .filter((path) => {
            if (!path || seen.has(path)) {
                return false;
            }
            seen.add(path);
            return true;
        });
};

const parseStoredPathField = (pathValue: unknown) => {
    const defaults = {
        folderPaths: [] as string[],
        viewMode: DEFAULT_LIBRARY_VIEW_MODE,
        titleField: DEFAULT_LIBRARY_TITLE_FIELD,
        subtitleField: DEFAULT_LIBRARY_SUBTITLE_FIELD,
    };

    if (typeof pathValue !== 'string') {
        return defaults;
    }

    const trimmed = pathValue.trim();
    if (!trimmed) {
        return defaults;
    }

    try {
        if (trimmed.startsWith('{')) {
            const parsed = JSON.parse(trimmed);
            return {
                folderPaths: normalizeFolderPaths(Array.isArray(parsed?.paths) ? parsed.paths : []),
                viewMode: parsed?.view_mode || defaults.viewMode,
                titleField: parsed?.title_field || defaults.titleField,
                subtitleField: parsed?.subtitle_field || defaults.subtitleField,
            };
        }
        if (trimmed.startsWith('[')) {
            const parsed = JSON.parse(trimmed);
            return {
                ...defaults,
                folderPaths: normalizeFolderPaths(Array.isArray(parsed) ? parsed : []),
            };
        }
    } catch (_error) {
        return {
            ...defaults,
            folderPaths: normalizeFolderPaths([trimmed]),
        };
    }

    return {
        ...defaults,
        folderPaths: normalizeFolderPaths([trimmed]),
    };
};

export const getLibraryConfig = (library: any) => {
    const stored = parseStoredPathField(library?.path);
    const explicitPaths = normalizeFolderPaths(Array.isArray(library?.folder_paths) ? library.folder_paths : []);

    return {
        folderPaths: explicitPaths.length > 0 ? explicitPaths : stored.folderPaths,
        viewMode: library?.view_mode || stored.viewMode,
        titleField: library?.title_field || stored.titleField,
        subtitleField: library?.subtitle_field || stored.subtitleField,
    };
};

export const buildLibraryPayload = (library: any, updates: {
    name: string;
    folderPaths: string[];
    viewMode: string;
    titleField: string;
    subtitleField: string;
}) => ({
    ...library,
    name: updates.name,
    path: updates.folderPaths[0] || '',
    folder_paths: normalizeFolderPaths(updates.folderPaths),
    view_mode: updates.viewMode || DEFAULT_LIBRARY_VIEW_MODE,
    title_field: updates.titleField || DEFAULT_LIBRARY_TITLE_FIELD,
    subtitle_field: updates.subtitleField || DEFAULT_LIBRARY_SUBTITLE_FIELD,
});

const toSegments = (path: string) => path.replace(/\\/g, '/').replace(/\/+$/, '').split('/').filter(Boolean);

export const formatLibraryPathLabel = (path: string, siblingPaths: string[] = []) => {
    const segments = toSegments(path);
    if (segments.length === 0) {
        return path;
    }

    const base = segments[segments.length - 1];
    const duplicateBaseCount = siblingPaths.filter((item) => toSegments(item).slice(-1)[0] === base).length;
    if (duplicateBaseCount > 1 && segments.length > 1) {
        return `${segments[segments.length - 2]}/${base}`;
    }

    return base;
};

export const getSortLabel = (field: string) => {
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
