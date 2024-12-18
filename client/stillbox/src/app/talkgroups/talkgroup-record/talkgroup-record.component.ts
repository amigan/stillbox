import { Component, ChangeDetectionStrategy, inject } from '@angular/core';
import {
  Talkgroup,
  TalkgroupUpdate,
  IconMap,
  iconMapping,
} from '../../talkgroup';
import { TalkgroupService } from '../talkgroups.service';
import { AlertRuleBuilderComponent } from './alert-rule-builder/alert-rule-builder.component';
import { CommonModule } from '@angular/common';
import { catchError, of } from 'rxjs';
import {
  ReactiveFormsModule,
  FormGroup,
  FormControl,
  FormsModule,
} from '@angular/forms';
import { Router, ActivatedRoute, ParamMap } from '@angular/router';
import { MatInputModule } from '@angular/material/input';
import { MatFormFieldModule } from '@angular/material/form-field';
import { MatCheckboxModule } from '@angular/material/checkbox';
import { MatChipInputEvent, MatChipsModule } from '@angular/material/chips';
import { MatIconModule } from '@angular/material/icon';

@Component({
  selector: 'talkgroup-record',
  standalone: true,
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
  ],
  templateUrl: './talkgroup-record.component.html',
  styleUrl: './talkgroup-record.component.scss',
})
export class TalkgroupRecordComponent {
  tg!: Talkgroup;
  iconMapping: IconMap = iconMapping;
  tgService: TalkgroupService = inject(TalkgroupService);
  tagsControl = new FormControl<string[]>([]);

  form!: FormGroup;

  constructor(
    private route: ActivatedRoute,
    private router: Router,
  ) {}

  removeTag(tag: string) {
    const idx = this.tg.tags.indexOf(tag);
    if (idx < 0) {
      return this.tg.tags;
    }

    this.tg.tags.splice(idx, 1);

    return [...this.tg.tags];
  }

  addTag(event: MatChipInputEvent) {
    const value = (event.value || '').trim();

    if (value) {
      this.tg.tags = [...this.tg.tags, value];
    }

    event.chipInput!.clear();
  }

  ngOnInit() {
    const sysId = this.route.snapshot.paramMap.get('sys');
    const tgId = this.route.snapshot.paramMap.get('tg');

    this.tgService
      .getTalkgroup(Number(sysId), Number(tgId))
      .subscribe((data: Talkgroup) => {
        this.tg = data;
        this.form = new FormGroup({
          name: new FormControl(this.tg.name),
          alpha_tag: new FormControl(this.tg.alpha_tag),
          tg_group: new FormControl(this.tg.tg_group),
          frequency: new FormControl(this.tg.frequency),
          alert: new FormControl(this.tg.alert),
          weight: new FormControl(this.tg.weight),
          icon: new FormControl(this.tg?.metadata?.icon ?? ''),
        });
        this.tagsControl.setValue(this.tg?.tags ?? []);
      });
  }

  submit() {
    let tgu: TalkgroupUpdate = <TalkgroupUpdate>{
      system_id: this.tg.system_id,
      tgid: this.tg.tgid,
      id: this.tg.id,
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
    if (this.tagsControl.dirty) {
      tgu.tags = this.tagsControl.value;
    }
    if (this.form.controls['icon'].dirty) {
      let iv: string = this.form.controls['icon'].value;
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
    this.tgService
      .putTalkgroup(tgu)
      .pipe(
        catchError(() => {
          return of(null);
        }),
      )
      .subscribe((event) => {
        this.router.navigate(['/talkgroups/']);
      });
  }
}
