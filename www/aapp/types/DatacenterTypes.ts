/// <reference path="../References.d.ts"/>
export const SYNC = 'datacenter.sync';
export const CHANGE = 'datacenter.change';

export interface Datacenter {
	id: string;
	organizations?: string[];
	name?: string;
}

export type Datacenters = Datacenter[];

export type DatacenterRo = Readonly<Datacenter>;
export type DatacentersRo = ReadonlyArray<DatacenterRo>;

export interface DatacenterDispatch {
	type: string;
	data?: {
		id?: string;
		datacenter?: Datacenter;
		datacenters?: Datacenters;
	};
}