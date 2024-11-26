import { Component, inject } from '@angular/core';
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
  Validators,
} from '@angular/forms';
import { Router, ActivatedRoute, ParamMap } from '@angular/router';
import { Observable } from 'rxjs';

@Component({
  selector: 'talkgroup-record',
  standalone: true,
  imports: [CommonModule, ReactiveFormsModule, AlertRuleBuilderComponent],
  templateUrl: './talkgroup-record.component.html',
  styleUrl: './talkgroup-record.component.css',
})
export class TalkgroupRecordComponent {
  tg!: Talkgroup;
  iconMapping: IconMap = iconMapping;
  tgService: TalkgroupService = inject(TalkgroupService);

  form!: FormGroup;

  constructor(
    private route: ActivatedRoute,
    private router: Router,
  ) {}

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
          icon: new FormControl(this.tg.icon),
        });
        console.log(this.tg.icon);
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
    if (this.form.controls['icon'].dirty) {
      if (tgu.metadata == null) {
        tgu.metadata = {};
      }
      if (this.tg.icon == null || this.tg.icon == '') {
        tgu.metadata = Object.assign(tgu.metadata, { icon: undefined });
      } else {
        tgu.metadata = Object.assign(tgu.metadata!, {
          icon: this.form.controls['icon'],
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
