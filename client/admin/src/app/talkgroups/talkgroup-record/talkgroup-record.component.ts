import { Component, inject } from '@angular/core';
import { Talkgroup, TalkgroupUpdate } from '../../talkgroup';
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
      });

    this.form = new FormGroup({
      name: new FormControl(''),
      alpha_tag: new FormControl(''),
      tg_group: new FormControl(''),
      frequency: new FormControl(0),
      alert: new FormControl(true),
      weight: new FormControl(1.0),
      icon: new FormControl(''),
    });
  }

  submit() {
    let tgu: TalkgroupUpdate = <TalkgroupUpdate>{
      system_id: this.tg.system_id,
      tgid: this.tg.tgid,
      id: this.tg.id,
    };
    if (this.form.controls['name'].dirty) {
      tgu.name = this.tg.name;
    }
    if (this.form.controls['alpha_tag'].dirty) {
      tgu.alpha_tag = this.tg.alpha_tag;
    }
    if (this.form.controls['tg_group'].dirty) {
      tgu.tg_group = this.tg.tg_group;
    }
    if (this.form.controls['frequency'].dirty) {
      tgu.frequency = this.tg.frequency;
    }
    if (this.form.controls['alert'].dirty) {
      tgu.alert = this.tg.alert;
    }
    if (this.form.controls['weight'].dirty) {
      tgu.weight = Number(this.tg.weight);
    }
    if (this.form.controls['icon'].dirty) {
      if (tgu.metadata == null) {
        tgu.metadata = {};
      }
      if (this.tg.icon == null || this.tg.icon == '') {
        tgu.metadata = Object.assign(tgu.metadata, {icon: undefined});
      } else {
      tgu.metadata = Object.assign(tgu.metadata!, { icon: this.tg.icon });
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
