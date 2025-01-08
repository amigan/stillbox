export interface CallRecord {
  id: string;
  call_date: Date;
  duration: number;
  system_id: number;
  tgid: number;
  incidents: number; // in incident
}
