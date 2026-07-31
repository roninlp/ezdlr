export namespace main {
	
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
	export class DownloadItem {
	    id: string;
	    url: string;
	    state: string;
	    destination: string;
	    addedAt: string;
	    gid: string;
	    totalBytes: number;
	    completedBytes: number;
	    downloadSpeed: number;
	
	    static createFrom(source: any = {}) {
	        return new DownloadItem(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.url = source["url"];
	        this.state = source["state"];
	        this.destination = source["destination"];
	        this.addedAt = source["addedAt"];
	        this.gid = source["gid"];
	        this.totalBytes = source["totalBytes"];
	        this.completedBytes = source["completedBytes"];
	        this.downloadSpeed = source["downloadSpeed"];
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
