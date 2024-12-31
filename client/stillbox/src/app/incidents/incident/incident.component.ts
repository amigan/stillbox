import { Component, computed, inject, ViewChild } from '@angular/core';
import { toSignal } from '@angular/core/rxjs-interop';
import { debounceTime } from 'rxjs/operators';
import {
  Talkgroup,
  TalkgroupUpdate,
  IconMap,
  iconMapping,
} from '../../talkgroup';
import { COMMA, ENTER } from '@angular/cdk/keycodes';
import { TalkgroupService } from '../../talkgroups/talkgroups.service';
import {
  MatAutocomplete,
  MatAutocompleteModule,
  MatAutocompleteSelectedEvent,
  MatAutocompleteActivatedEvent,
} from '@angular/material/autocomplete';
import { CommonModule, DatePipe } from '@angular/common';
import { BehaviorSubject, catchError, of, Subscription } from 'rxjs';
import { shareReplay } from 'rxjs/operators';
import { Observable } from 'rxjs';
import {
  ReactiveFormsModule,
  FormGroup,
  FormControl,
  FormsModule,
} from '@angular/forms';
import { Router, ActivatedRoute } from '@angular/router';
import { MatInputModule } from '@angular/material/input';
import { MatFormFieldModule } from '@angular/material/form-field';
import { MatCheckboxModule } from '@angular/material/checkbox';
import { MatIconModule } from '@angular/material/icon';
import { IncidentsService } from '../incidents.service';
import { IncidentRecord } from '../../incidents';
import { MatCardModule } from '@angular/material/card';
import { FmtDatePipe } from '../incidents.component';

@Component({
  selector: 'app-incident',
  imports: [
    CommonModule,
    ReactiveFormsModule,
    FormsModule,
    MatInputModule,
    MatFormFieldModule,
    MatCheckboxModule,
    MatIconModule,
    MatCardModule,
    FmtDatePipe,
  ],
  templateUrl: './incident.component.html',
  styleUrl: './incident.component.scss',
})
export class IncidentComponent {
  inc$!: Observable<IncidentRecord>;
  subscriptions: Subscription = new Subscription();

  constructor(
    private route: ActivatedRoute,
    private incSvc: IncidentsService,
  ) {}

  saveIncName(ev: Event) {}

  ngOnInit() {
    const incID = this.route.snapshot.paramMap.get('id')!;
    this.inc$ = this.incSvc.getIncident(incID);
  }

  ngOnDestroy() {
    this.subscriptions.unsubscribe();
  }
}
