/// <reference path="../References.d.ts"/>
export const SYNC = 'firewall.sync';
export const TRAVERSE = 'firewall.traverse';
export const FILTER = 'firewall.filter';
export const CHANGE = 'firewall.change';

export interface Rule {
	protocol: string;
	port?: string;
	source_ips?: string[];
}

export interface Firewall {
	id?: string;
	name?: string;
	organization?: string;
	network_roles?: string[];
	ingress?: Rule[];
}

export interface Filter {
	name?: string;
}

export type Firewalls = Firewall[];

export type FirewallRo = Readonly<Firewall>;
export type FirewallsRo = ReadonlyArray<FirewallRo>;

export interface FirewallDispatch {
	type: string;
	data?: {
		id?: string;
		firewall?: Firewall;
		firewalls?: Firewalls;
		page?: number;
		pageCount?: number;
		filter?: Filter;
		count?: number;
	};
}