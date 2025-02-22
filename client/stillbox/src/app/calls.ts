export interface CallRecord {
  id: string;
  callDate: Date;
  audioURL: string | null;
  duration: number;
  systemId: number;
  talkerAlias: string | null;
  tgid: number;
  incidents: number; // in incident
}

export interface CallStats {
  stats: CallStatsRecord[];
  interval: string;
}

export interface CallStatsRecord {
  count: number;
  time: Date;
}
