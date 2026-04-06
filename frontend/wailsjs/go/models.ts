export namespace gorm {
	
	export class DeletedAt {
	    // Go type: time
	    Time: any;
	    Valid: boolean;
	
	    static createFrom(source: any = {}) {
	        return new DeletedAt(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.Time = this.convertValues(source["Time"], null);
	        this.Valid = source["Valid"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}

}

export namespace main {
	
	export class DesktopSettings {
	    player_path: string;
	    use_external_player: boolean;
	    loop_playback: boolean;
	    read_file_info: boolean;
	    theme: string;
	    poster_radius: number;
	    backdrop_blur: number;
	    min_window_width: number;
	    show_subtitle_tag: boolean;
	    show_resolution_tag: boolean;
	    show_count_tag: boolean;
	    show_genre_in_list: boolean;
	    show_series_in_list: boolean;
	    static_loading: boolean;
	    hotkey: string;
	    min_to_tray: boolean;
	    max_no_taskbar: boolean;
	    show_prompt: boolean;
	    start_with_os: boolean;
	    skip_no_nfo: boolean;
	    get_resolution: boolean;
	    use_everything: boolean;
	    everything_addr: string;
	    scan_from_video_dir: boolean;
	    emby_enabled: boolean;
	    emby_user: string;
	    emby_url: string;
	    emby_api_key: string;
	
	    static createFrom(source: any = {}) {
	        return new DesktopSettings(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.player_path = source["player_path"];
	        this.use_external_player = source["use_external_player"];
	        this.loop_playback = source["loop_playback"];
	        this.read_file_info = source["read_file_info"];
	        this.theme = source["theme"];
	        this.poster_radius = source["poster_radius"];
	        this.backdrop_blur = source["backdrop_blur"];
	        this.min_window_width = source["min_window_width"];
	        this.show_subtitle_tag = source["show_subtitle_tag"];
	        this.show_resolution_tag = source["show_resolution_tag"];
	        this.show_count_tag = source["show_count_tag"];
	        this.show_genre_in_list = source["show_genre_in_list"];
	        this.show_series_in_list = source["show_series_in_list"];
	        this.static_loading = source["static_loading"];
	        this.hotkey = source["hotkey"];
	        this.min_to_tray = source["min_to_tray"];
	        this.max_no_taskbar = source["max_no_taskbar"];
	        this.show_prompt = source["show_prompt"];
	        this.start_with_os = source["start_with_os"];
	        this.skip_no_nfo = source["skip_no_nfo"];
	        this.get_resolution = source["get_resolution"];
	        this.use_everything = source["use_everything"];
	        this.everything_addr = source["everything_addr"];
	        this.scan_from_video_dir = source["scan_from_video_dir"];
	        this.emby_enabled = source["emby_enabled"];
	        this.emby_user = source["emby_user"];
	        this.emby_url = source["emby_url"];
	        this.emby_api_key = source["emby_api_key"];
	    }
	}
	export class StatsItem {
	    name: string;
	    count: number;
	    image: string;
	    filter_value: string;
	
	    static createFrom(source: any = {}) {
	        return new StatsItem(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.count = source["count"];
	        this.image = source["image"];
	        this.filter_value = source["filter_value"];
	    }
	}

}

export namespace model {
	
	export class Library {
	    id: string;
	    name: string;
	    path: string;
	    type: string;
	    // Go type: time
	    last_scan?: any;
	    folder_paths?: string[];
	    view_mode?: string;
	    title_field?: string;
	    subtitle_field?: string;
	    media_count?: number;
	    prefer_local_nfo: boolean;
	    min_file_size: number;
	    enable_file_filter: boolean;
	    metadata_lang: string;
	    allow_adult_content: boolean;
	    auto_download_sub: boolean;
	    metadata_mode: string;
	    enable_file_watch: boolean;
	    // Go type: time
	    created_at: any;
	    // Go type: time
	    updated_at: any;
	
	    static createFrom(source: any = {}) {
	        return new Library(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.path = source["path"];
	        this.type = source["type"];
	        this.last_scan = this.convertValues(source["last_scan"], null);
	        this.folder_paths = source["folder_paths"];
	        this.view_mode = source["view_mode"];
	        this.title_field = source["title_field"];
	        this.subtitle_field = source["subtitle_field"];
	        this.media_count = source["media_count"];
	        this.prefer_local_nfo = source["prefer_local_nfo"];
	        this.min_file_size = source["min_file_size"];
	        this.enable_file_filter = source["enable_file_filter"];
	        this.metadata_lang = source["metadata_lang"];
	        this.allow_adult_content = source["allow_adult_content"];
	        this.auto_download_sub = source["auto_download_sub"];
	        this.metadata_mode = source["metadata_mode"];
	        this.enable_file_watch = source["enable_file_watch"];
	        this.created_at = this.convertValues(source["created_at"], null);
	        this.updated_at = this.convertValues(source["updated_at"], null);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class MediaActor {
	    id: string;
	    name: string;
	
	    static createFrom(source: any = {}) {
	        return new MediaActor(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	    }
	}
	export class Series {
	    id: string;
	    library_id: string;
	    title: string;
	    orig_title: string;
	    year: number;
	    overview: string;
	    poster_path: string;
	    backdrop_path: string;
	    rating: number;
	    genres: string;
	    folder_path: string;
	    season_count: number;
	    episode_count: number;
	    tmdb_id: number;
	    douban_id: string;
	    bangumi_id: number;
	    country: string;
	    language: string;
	    studio: string;
	    // Go type: time
	    created_at: any;
	    // Go type: time
	    updated_at: any;
	    episodes?: Media[];
	
	    static createFrom(source: any = {}) {
	        return new Series(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.library_id = source["library_id"];
	        this.title = source["title"];
	        this.orig_title = source["orig_title"];
	        this.year = source["year"];
	        this.overview = source["overview"];
	        this.poster_path = source["poster_path"];
	        this.backdrop_path = source["backdrop_path"];
	        this.rating = source["rating"];
	        this.genres = source["genres"];
	        this.folder_path = source["folder_path"];
	        this.season_count = source["season_count"];
	        this.episode_count = source["episode_count"];
	        this.tmdb_id = source["tmdb_id"];
	        this.douban_id = source["douban_id"];
	        this.bangumi_id = source["bangumi_id"];
	        this.country = source["country"];
	        this.language = source["language"];
	        this.studio = source["studio"];
	        this.created_at = this.convertValues(source["created_at"], null);
	        this.updated_at = this.convertValues(source["updated_at"], null);
	        this.episodes = this.convertValues(source["episodes"], Media);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class Media {
	    id: string;
	    library_id: string;
	    title: string;
	    orig_title: string;
	    year: number;
	    overview: string;
	    poster_path: string;
	    backdrop_path: string;
	    rating: number;
	    runtime: number;
	    genres: string;
	    file_path: string;
	    file_size: number;
	    media_type: string;
	    video_codec: string;
	    audio_codec: string;
	    resolution: string;
	    duration: number;
	    subtitle_paths: string;
	    stream_url: string;
	    tmdb_id: number;
	    douban_id: string;
	    bangumi_id: number;
	    country: string;
	    language: string;
	    tagline: string;
	    studio: string;
	    trailer_url: string;
	    stack_group: string;
	    stack_order: number;
	    version_tag: string;
	    version_group: string;
	    nfo_extra_fields: string;
	    release_date_normalized: string;
	    scrape_status: string;
	    scrape_attempts: number;
	    // Go type: time
	    last_scrape_at?: any;
	    series_id: string;
	    season_num: number;
	    episode_num: number;
	    episode_title: string;
	    // Go type: time
	    created_at: any;
	    // Go type: time
	    updated_at: any;
	    series?: Series;
	    actor: string;
	    actors?: MediaActor[];
	    is_favorite: boolean;
	    is_watched: boolean;
	
	    static createFrom(source: any = {}) {
	        return new Media(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.library_id = source["library_id"];
	        this.title = source["title"];
	        this.orig_title = source["orig_title"];
	        this.year = source["year"];
	        this.overview = source["overview"];
	        this.poster_path = source["poster_path"];
	        this.backdrop_path = source["backdrop_path"];
	        this.rating = source["rating"];
	        this.runtime = source["runtime"];
	        this.genres = source["genres"];
	        this.file_path = source["file_path"];
	        this.file_size = source["file_size"];
	        this.media_type = source["media_type"];
	        this.video_codec = source["video_codec"];
	        this.audio_codec = source["audio_codec"];
	        this.resolution = source["resolution"];
	        this.duration = source["duration"];
	        this.subtitle_paths = source["subtitle_paths"];
	        this.stream_url = source["stream_url"];
	        this.tmdb_id = source["tmdb_id"];
	        this.douban_id = source["douban_id"];
	        this.bangumi_id = source["bangumi_id"];
	        this.country = source["country"];
	        this.language = source["language"];
	        this.tagline = source["tagline"];
	        this.studio = source["studio"];
	        this.trailer_url = source["trailer_url"];
	        this.stack_group = source["stack_group"];
	        this.stack_order = source["stack_order"];
	        this.version_tag = source["version_tag"];
	        this.version_group = source["version_group"];
	        this.nfo_extra_fields = source["nfo_extra_fields"];
	        this.release_date_normalized = source["release_date_normalized"];
	        this.scrape_status = source["scrape_status"];
	        this.scrape_attempts = source["scrape_attempts"];
	        this.last_scrape_at = this.convertValues(source["last_scrape_at"], null);
	        this.series_id = source["series_id"];
	        this.season_num = source["season_num"];
	        this.episode_num = source["episode_num"];
	        this.episode_title = source["episode_title"];
	        this.created_at = this.convertValues(source["created_at"], null);
	        this.updated_at = this.convertValues(source["updated_at"], null);
	        this.series = this.convertValues(source["series"], Series);
	        this.actor = source["actor"];
	        this.actors = this.convertValues(source["actors"], MediaActor);
	        this.is_favorite = source["is_favorite"];
	        this.is_watched = source["is_watched"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	

}

export namespace service {
	
	export class NFOEditorData {
	    nfo_path: string;
	    title: string;
	    code: string;
	    release_date: string;
	    director: string;
	    series: string;
	    publisher: string;
	    maker: string;
	    genres: string;
	    actors: string;
	    plot: string;
	    runtime: string;
	    file_size: string;
	    resolution: string;
	    video_codec: string;
	    rating: string;
	
	    static createFrom(source: any = {}) {
	        return new NFOEditorData(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.nfo_path = source["nfo_path"];
	        this.title = source["title"];
	        this.code = source["code"];
	        this.release_date = source["release_date"];
	        this.director = source["director"];
	        this.series = source["series"];
	        this.publisher = source["publisher"];
	        this.maker = source["maker"];
	        this.genres = source["genres"];
	        this.actors = source["actors"];
	        this.plot = source["plot"];
	        this.runtime = source["runtime"];
	        this.file_size = source["file_size"];
	        this.resolution = source["resolution"];
	        this.video_codec = source["video_codec"];
	        this.rating = source["rating"];
	    }
	}

}

