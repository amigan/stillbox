import { Routes } from '@angular/router';

import { HomeComponent } from './home/home.component';
import { LoginComponent } from './login/login.component';
import { TalkgroupsComponent } from './talkgroups/talkgroups.component';
import { TalkgroupRecordComponent } from './talkgroups/talkgroup-record/talkgroup-record.component';
import { CallsComponent } from './calls/calls.component';
import { IncidentsComponent } from './incidents/incidents.component';
import { AlertsComponent } from './alerts/alerts.component';
import { ImportComponent } from './talkgroups/import/import.component';
import { AuthGuard } from './auth.guard';

export const routes: Routes = [
  { path: 'login', component: LoginComponent },
  { path: '', component: HomeComponent, canActivate: [AuthGuard] },
  { path: '**', redirectTo: '' },
  { path: 'talkgroups', component: TalkgroupsComponent },
  { path: 'talkgroups/import', component: ImportComponent },
  { path: 'talkgroups/:sys/:tg', component: TalkgroupRecordComponent },
  { path: 'calls', component: CallsComponent },
  { path: 'incidents', component: IncidentsComponent },
  { path: 'alerts', component: AlertsComponent },
];
