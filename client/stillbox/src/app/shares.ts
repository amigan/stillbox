import { IncidentRecord } from './incidents';
import { CallRecord } from './calls';
export type ShareType = IncidentRecord | CallRecord | null;
export interface Share {
  id: string;
  type: string;
  sharedItem: ShareType;
}
