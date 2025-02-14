export interface CallRecord {
  id: string;
  callDate: Date;
  audioURL: string | null;
  duration: number;
  systemId: number;
  tgid: number;
  incidents: number; // in incident
}
