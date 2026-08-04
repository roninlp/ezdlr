export namespace main {
	
	export class DownloadItem {
	    id: string;
	    url: string;
	    gid?: string;
	    state: string;
	    destination: string;
	    path?: string;
	    addedAt: string;
	    totalBytes: number;
	    completedBytes: number;
	    downloadSpeed: number;
	    attempts: number;
	
	    static createFrom(source: any = {}) {
	        return new DownloadItem(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.url = source["url"];
	        this.gid = source["gid"];
	        this.state = source["state"];
	        this.destination = source["destination"];
	        this.path = source["path"];
	        this.addedAt = source["addedAt"];
	        this.totalBytes = source["totalBytes"];
	        this.completedBytes = source["completedBytes"];
	        this.downloadSpeed = source["downloadSpeed"];
	        this.attempts = source["attempts"];
	    }
	}
	export class ClipboardResult {
	    url: string;
	    status: string;
	    reason?: string;
	    item?: DownloadItem;
	
	    static createFrom(source: any = {}) {
	        return new ClipboardResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.url = source["url"];
	        this.status = source["status"];
	        this.reason = source["reason"];
	        this.item = this.convertValues(source["item"], DownloadItem);
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
	export class ClipboardBatchResult {
	    results: ClipboardResult[];
	
	    static createFrom(source: any = {}) {
	        return new ClipboardBatchResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.results = this.convertValues(source["results"], ClipboardResult);
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
	
	export class ClipboardURL {
	    url: string;
	    reason?: string;
	
	    static createFrom(source: any = {}) {
	        return new ClipboardURL(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.url = source["url"];
	        this.reason = source["reason"];
	    }
	}
	export class ClipboardReview {
	    accepted: ClipboardURL[];
	    duplicates: ClipboardURL[];
	    rejected: ClipboardURL[];
	
	    static createFrom(source: any = {}) {
	        return new ClipboardReview(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.accepted = this.convertValues(source["accepted"], ClipboardURL);
	        this.duplicates = this.convertValues(source["duplicates"], ClipboardURL);
	        this.rejected = this.convertValues(source["rejected"], ClipboardURL);
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
	
	export class Configuration {
	    downloadDirectory: string;
	    activeLimit: number;
	    connections: number;
	    maxRetries: number;
	
	    static createFrom(source: any = {}) {
	        return new Configuration(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.downloadDirectory = source["downloadDirectory"];
	        this.activeLimit = source["activeLimit"];
	        this.connections = source["connections"];
	        this.maxRetries = source["maxRetries"];
	    }
	}
	
	export class ServiceSnapshot {
	    items: DownloadItem[];
	    configuration: Configuration;
	
	    static createFrom(source: any = {}) {
	        return new ServiceSnapshot(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.items = this.convertValues(source["items"], DownloadItem);
	        this.configuration = this.convertValues(source["configuration"], Configuration);
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

