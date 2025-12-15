import { CallRecord } from './calls';

export interface IncidentCall extends CallRecord {
  notes: Object | null;
}

export interface IncidentRecord {
  id: string;
  name: string | null;
  description: string | null;
  startTime: Date | null;
  endTime: Date | null;
  location: Object | null;
  metadata: Object | null;
  calls: IncidentCall[] | null;
  owner: string | null;
  callCount: number | null;
}

export interface IncidentCalls {
  calls: IncidentCall[];
  count: number;
}
