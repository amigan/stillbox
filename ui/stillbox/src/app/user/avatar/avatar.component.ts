import { Component, input, signal, Signal } from '@angular/core';
import { User, UserService } from '../user.service';
import { Subscription } from 'rxjs';

@Component({
  selector: 'avatar',
  imports: [],
  templateUrl: './avatar.component.html',
  styleUrl: './avatar.component.scss',
})
export class AvatarComponent {
  user = input<User | string | undefined>();
  avatar = signal<string>('');
  private sub!: Subscription;
  public circleColor: string = '#bbff00';
  colors = [
    '#4419CD',
    '#E8035F',
    '#86F103',
    '#FFD403',
    '#8064DA',
    '#EE5C97',
    '#B1F45F',
    '#FFE463',
  ];

  constructor(private userSvc: UserService) {}

  ngOnInit() {
    const u = this.user();
    let username = '';
    if (typeof u == 'string') {
      username = this.user() as string;
      this.sub = this.userSvc.getUser(username).subscribe((u) => {
        this.avatar.set(this.setInitials(u));
      });
      this.setColor(username);
    } else if (u == undefined) {
      this.sub = this.userSvc
        .getSelf()
        .subscribe((u) => this.avatar.set(this.setInitials(u)));
    } else {
      // User
      const us = u as User;
      this.setInitials(us);
      this.setColor(us.username);
    }
  }

  hashString(s: string): number {
    return s
      .split('')
      .map((char) => char.charCodeAt(0))
      .reduce((a, b) => a + b, 0);
  }

  setColor(username: string) {
    this.circleColor =
      this.colors[this.hashString(username) % this.colors.length];
  }

  setInitials(u: User): string {
    let uname = u.realName;
    if (uname == null || uname == undefined) {
      uname = u.username;
      if (uname == null || uname == undefined) {
        return '<>';
      }
    } else {
      let names = uname.split(' ');
      return names[0][0].toUpperCase() + names[1][0].toUpperCase();
    }
    return uname![0].toUpperCase() + uname![1].toUpperCase();
  }

  ngOnDestroy() {
    this.sub.unsubscribe();
  }
}
