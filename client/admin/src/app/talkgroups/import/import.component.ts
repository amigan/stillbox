import { Component, inject } from '@angular/core';
import { TalkgroupService } from '../talkgroups.service';
import { Talkgroup } from '../../talkgroup';
import { FormGroup, FormControl, ReactiveFormsModule, FormsModule  } from '@angular/forms';
import { CommonModule } from '@angular/common';
import { Router, ActivatedRoute } from '@angular/router';
import { catchError, of } from 'rxjs';

@Component({
  selector: 'app-import',
  standalone: true,
  imports: [CommonModule, ReactiveFormsModule, FormsModule ],
  templateUrl: './import.component.html',
  styleUrl: './import.component.css',
})
export class ImportComponent {
  tgService: TalkgroupService = inject(TalkgroupService);
  form!: FormGroup;
  tgs!: Talkgroup[];
  selectAll: boolean = false;

  constructor(
    private route: ActivatedRoute,
    private router: Router,
  ) {}

  ngOnInit() {
    this.form = new FormGroup({
      contents: new FormControl(''),
      systemID: new FormControl(0),
    });
  }
  submit() {
    let content = this.form.controls['contents'].value;
    let sysID = Number(this.form.controls['systemID'].value);
    this.tgService.importRR(sysID, content)
      .pipe(
        catchError(() => {
          return of(null);
        }),
      )
      .subscribe((event) => {
        this.tgs = event!;
        //this.router.navigate(['/talkgroups/']);
      });
  }

  isAllSelected() {
  }

  save() {
  }
}
