import { Routes } from '@angular/router';

import { AuthGuard } from './auth.guard';

export const routes: Routes = [
  {
    path: 'login',
    loadComponent: () =>
      import('./login/login.component').then((m) => m.LoginComponent),
  },
  {
    path: '',
    canActivateChild: [AuthGuard],
    children: [
      {
        path: '',
        loadComponent: () =>
          import('./home/home.component').then((m) => m.HomeComponent),
        pathMatch: 'full',
        data: { title: 'Home' },
      },
      {
        path: 'talkgroups',
        loadComponent: () =>
          import('./talkgroups/talkgroups.component').then(
            (m) => m.TalkgroupsComponent,
          ),
        data: { title: 'Talkgroups' },
      },
      {
        path: 'talkgroups/import',
        loadComponent: () =>
          import('./talkgroups/import/import.component').then(
            (m) => m.ImportComponent,
          ),
        data: { title: 'Import Talkgroups' },
      },
      {
        path: 'talkgroups/export',
        loadComponent: () =>
          import('./talkgroups/export/export.component').then(
            (m) => m.ExportComponent,
          ),
        data: { title: 'Export Talkgroups' },
      },
      {
        path: 'talkgroups/:sys/:tg',
        loadComponent: () =>
          import(
            './talkgroups/talkgroup-record/talkgroup-record.component'
          ).then((m) => m.TalkgroupRecordComponent),
        data: { title: 'Edit Talkgroup' },
      },
      {
        path: 'calls',
        loadComponent: () =>
          import('./calls/calls.component').then((m) => m.CallsComponent),
        data: { title: 'Calls' },
      },
      {
        path: 'incidents',
        loadComponent: () =>
          import('./incidents/incidents.component').then(
            (m) => m.IncidentsComponent,
          ),
        data: { title: 'Incidents' },
      },
      {
        path: 'alerts',
        loadComponent: () =>
          import('./alerts/alerts.component').then((m) => m.AlertsComponent),
        data: { title: 'Alerts' },
      },
    ],
  },
];
