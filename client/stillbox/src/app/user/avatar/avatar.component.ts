import { Component } from '@angular/core';
import { UserService } from '../user.service';

@Component({
  selector: 'avatar',
  imports: [],
  templateUrl: './avatar.component.html',
  styleUrl: './avatar.component.scss',
})
export class AvatarComponent {
  constructor(public userSvc: UserService) {}

  getInitials(): string {
    let uname = this.userSvc.selfUser()?.realName;
    if (uname == null || uname == undefined) {
      uname = this.userSvc.selfUser()?.username;
      if (uname == null || uname == undefined) {
        return "";
      }
    } else {
      let names = uname!.split(' ');
      return names[0][0].toUpperCase() + names[1][0].toUpperCase();
    }
    return uname![0].toUpperCase() + uname![1].toUpperCase();
  }
}
