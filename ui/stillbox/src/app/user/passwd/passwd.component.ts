import { Component } from '@angular/core';
import {
  AbstractControl,
  FormControl,
  FormGroup,
  ReactiveFormsModule,
  ValidationErrors,
  Validators,
} from '@angular/forms';
import { MatFormFieldModule } from '@angular/material/form-field';
import { MatInputModule } from '@angular/material/input';
import { UserService } from '../user.service';
import { catchError } from 'rxjs';
import { ErrorsService } from '../../errors/errors.service';
import { HttpErrorResponse } from '@angular/common/http';

@Component({
  selector: 'password-change',
  imports: [ReactiveFormsModule, MatFormFieldModule, MatInputModule],
  templateUrl: './passwd.component.html',
  styleUrl: './passwd.component.scss',
})
export class PasswdComponent {
  public form: FormGroup;
  constructor(
    public userSvc: UserService,
    private errorSvc: ErrorsService,
  ) {
    this.form = new FormGroup({
      oldPassword: new FormControl('', [Validators.required]),
      newPassword: new FormControl('', [Validators.required]),
      newAgain: new FormControl('', [
        Validators.required,
        this.validateSamePassword,
      ]),
    });
  }

  private validateSamePassword(
    control: AbstractControl,
  ): ValidationErrors | null {
    const newPassword = control.parent?.get('newPassword');
    const newAgain = control.parent?.get('newAgain');
    return newPassword?.value == newAgain?.value ? null : { notSame: true };
  }

  onSubmit() {
    if (this.form.invalid) {
      return;
    }

    this.userSvc
      .changePassword(
        this.form.controls['oldPassword'].value!,
        this.form.controls['newPassword'].value!,
      )
      .subscribe({
        next: () => {
          alert('Password changed!');
        },
        error: (err: HttpErrorResponse) => {
          this.errorSvc.show(err.error.error);
        },
      });
  }
}
