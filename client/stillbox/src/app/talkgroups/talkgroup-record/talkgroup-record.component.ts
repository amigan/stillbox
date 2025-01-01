import { Component, computed, inject, ViewChild } from '@angular/core';
import { toSignal } from '@angular/core/rxjs-interop';
import { debounceTime, tap } from 'rxjs/operators';
import {
  Talkgroup,
  TalkgroupUpdate,
  IconMap,
  iconMapping,
  TGID,
} from '../../talkgroup';
import { COMMA, ENTER } from '@angular/cdk/keycodes';
import { TalkgroupService } from '../talkgroups.service';
import { AlertRuleBuilderComponent } from './alert-rule-builder/alert-rule-builder.component';
import {
  MatAutocomplete,
  MatAutocompleteModule,
  MatAutocompleteSelectedEvent,
  MatAutocompleteActivatedEvent,
} from '@angular/material/autocomplete';
import { CommonModule } from '@angular/common';
import { catchError, of, Subscription } from 'rxjs';
import { shareReplay } from 'rxjs/operators';
import { Observable } from 'rxjs';
import {
  ReactiveFormsModule,
  FormGroup,
  FormControl,
  FormsModule,
} from '@angular/forms';
import { MatInputModule } from '@angular/material/input';
import { MatFormFieldModule } from '@angular/material/form-field';
import { MatCheckboxModule } from '@angular/material/checkbox';
import { MatChipInputEvent, MatChipsModule } from '@angular/material/chips';
import { MatIconModule } from '@angular/material/icon';
import {
  MAT_DIALOG_DATA,
  MatDialogActions,
  MatDialogContent,
  MatDialogRef,
  MatDialogTitle,
} from '@angular/material/dialog';
import { BrowserModule } from '@angular/platform-browser';
import { MatProgressSpinnerModule } from '@angular/material/progress-spinner';
import { MatButtonModule } from '@angular/material/button';

@Component({
  selector: 'talkgroup-record',
  imports: [
    CommonModule,
    ReactiveFormsModule,
    AlertRuleBuilderComponent,
    FormsModule,
    MatInputModule,
    MatFormFieldModule,
    MatCheckboxModule,
    MatChipsModule,
    MatIconModule,
    MatAutocompleteModule,
    MatDialogTitle,
    MatDialogContent,
    MatDialogActions,
    MatProgressSpinnerModule,
    MatButtonModule,
  ],
  templateUrl: './talkgroup-record.component.html',
  styleUrl: './talkgroup-record.component.scss',
})
export class TalkgroupRecordComponent {
  dialogRef = inject(MatDialogRef<TalkgroupRecordComponent>);
  tgid = inject<TGID>(MAT_DIALOG_DATA);
  tg$!: Observable<Talkgroup>;
  tg!: Talkgroup;
  iconMapping: IconMap = iconMapping;
  tgService: TalkgroupService = inject(TalkgroupService);
  allTags = <string[]>[];
  readonly separatorKeysCodes: number[] = [ENTER, COMMA];
  readonly filteredTags = computed(() => {
    const currentTag = this.tagInputSig()?.toLowerCase() ?? '';
    return currentTag
      ? this.allTags.filter((tag) => tag.toLowerCase().includes(currentTag))
      : this.allTags.slice();
  });
  readonly _allTags: Observable<string[]>;
  form = new FormGroup({
    name: new FormControl(''),
    alpha_tag: new FormControl(''),
    tg_group: new FormControl(''),
    frequency: new FormControl(0),
    alert: new FormControl(false),
    weight: new FormControl(0.0),
    icon: new FormControl(''),
    tagInput: new FormControl(''),
    tagsControl: new FormControl<string[]>([]),
  });
  tagInputSig = toSignal(
    this.form.get('tagInput')!.valueChanges.pipe(debounceTime(300)) ?? of(null),
    {},
  );
  @ViewChild('auto') autocomp!: MatAutocomplete;
  active: string | null = null;
  subscriptions = new Subscription();

  constructor() {
    this._allTags = this.tgService.allTags().pipe(shareReplay());
  }

  removeTag(tag: string) {
    const idx = this.tg.tags.indexOf(tag);
    if (idx < 0) {
      return this.tg.tags;
    }

    this.tg.tags.splice(idx, 1);

    return [...this.tg.tags];
  }

  addTagEv(event: MatChipInputEvent) {
    if (this.active != null) {
      // this is a hack
      this.addTag(this.active);
      this.active = null;
      event.chipInput!.clear();
      return;
    }

    const value = (event.value || '').trim();
    this.addTag(value);

    event.chipInput!.clear();
  }

  activated(event: MatAutocompleteActivatedEvent) {
    this.active = event.option?.value;
  }

  addTag(tag: string) {
    const idx = this.tg.tags.indexOf(tag);
    if (idx > -1) {
      return;
    }

    if (tag) {
      this.tg.tags = [...this.tg.tags, tag];
    }
  }

  selected(event: any) {
    let ev = event as MatAutocompleteSelectedEvent;
    this.addTag(ev.option.viewValue);
    ev.option.deselect();
    this.form.controls['tagInput'].reset();
  }

  ngOnInit() {
    this.tg$ = this.tgService
      .getTalkgroup(Number(this.tgid.sys), Number(this.tgid.tg))
      .pipe(
        tap((tg) => {
          this.form.patchValue(tg);
          this.form.controls['tagInput'].setValue('');
          this.form.controls['tagsControl'].setValue(this.tg?.tags ?? []);
        }),
      );
    /*
    this.subscriptions.add(
      this.tgService
        .getTalkgroup(Number(this.tgid.sys), Number(this.tgid.tg))
        .subscribe((data: Talkgroup) => {
          this.tg = data;
          this.form.controls['name'].setValue(this.tg.name);
          this.form.controls['alpha_tag'].setValue(this.tg.alpha_tag);
          this.form.controls['tg_group'].setValue(this.tg.tg_group);
          this.form.controls['frequency'].setValue(this.tg.frequency);
          this.form.controls['alert'].setValue(this.tg.alert);
          this.form.controls['weight'].setValue(this.tg.weight);
          this.form.controls['icon'].setValue(this.tg?.metadata?.icon ?? '');
          this.form.controls['tagInput'].setValue('');
          this.form.controls['tagsControl'].setValue(this.tg?.tags ?? []);
        }),
    ); */
    this.subscriptions.add(
      this._allTags.subscribe((event) => {
        this.allTags = event;
      }),
    );
  }

  ngOnDestroy() {
    this.subscriptions.unsubscribe();
  }

  save() {
    let tgu: TalkgroupUpdate = <TalkgroupUpdate>{
      system_id: this.tgid.sys,
      tgid: this.tgid.tg,
    };
    if (this.form.controls['name'].dirty) {
      tgu.name = this.form.controls['name'].value;
    }
    if (this.form.controls['alpha_tag'].dirty) {
      tgu.alpha_tag = this.form.controls['alpha_tag'].value;
    }
    if (this.form.controls['tg_group'].dirty) {
      tgu.tg_group = this.form.controls['tg_group'].value;
    }
    if (this.form.controls['frequency'].dirty) {
      tgu.frequency = this.form.controls['frequency'].value;
    }
    if (this.form.controls['alert'].dirty) {
      tgu.alert = this.form.controls['alert'].value;
    }
    if (this.form.controls['weight'].dirty) {
      tgu.weight = Number(this.form.controls['weight'].value);
    }
    if (this.form.controls['tagsControl'].dirty) {
      tgu.tags = this.form.controls['tagsControl'].value;
    }
    if (this.form.controls['icon'].dirty) {
      let iv: string = this.form.controls['icon'].value ?? '';
      if (tgu.metadata == null) {
        tgu.metadata = {};
      }
      if (iv == '') {
        tgu.metadata = Object.assign(tgu.metadata, { icon: undefined });
      } else {
        tgu.metadata = Object.assign(tgu.metadata!, {
          icon: iv,
        });
      }
    }
    this.subscriptions.add(
      this.tgService
        .putTalkgroup(tgu)
        .pipe(
          catchError(() => {
            return of(null);
          }),
        )
        .subscribe((newTG) => {
          this.dialogRef.close(newTG);
        }),
    );
  }

  cancel() {
    this.dialogRef.close();
  }
}
