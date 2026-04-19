import { Component } from '@angular/core';
import { PasswdComponent } from '../passwd/passwd.component';
import { AuthService } from '../../login/auth.service';
import { UserService } from '../user.service';
import { PushSubscribeComponent } from '../../notify/subscribe/subscribe.component';

@Component({
  selector: 'app-profile',
  imports: [PasswdComponent, PushSubscribeComponent],
  templateUrl: './profile.component.html',
  styleUrl: './profile.component.scss',
})
export class ProfileComponent {
  constructor(public userSvc: UserService) {}
}
