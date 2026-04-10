import { TestBed } from '@angular/core/testing';
import { HttpClient } from '@angular/common/http';
import { of } from 'rxjs';

import { UserService } from './user.service';

describe('UserService', () => {
  let service: UserService;
  let httpSpy: jasmine.SpyObj<HttpClient>;
  const mockUser = {
    uid: 1,
    username: 'demo',
    realName: 'Demo User',
    email: 'demo@example.com',
    roles: ['user'],
    lastLoginAt: null,
    lastLoginFrom: null,
    isAdmin: false,
  };

  beforeEach(() => {
    httpSpy = jasmine.createSpyObj<HttpClient>('HttpClient', ['get']);
    httpSpy.get.and.returnValue(of(mockUser));
    TestBed.configureTestingModule({
      providers: [{ provide: HttpClient, useValue: httpSpy }],
    });
    service = TestBed.inject(UserService);
  });

  it('should be created', () => {
    expect(service).toBeTruthy();
  });

  it('should request self user only once', () => {
    service.getSelf().subscribe();
    service.getSelf().subscribe();
    service.getSelf().subscribe();

    expect(httpSpy.get).toHaveBeenCalledTimes(1);
    expect(httpSpy.get).toHaveBeenCalledWith('/api/user/');
  });
});
