import { HttpClient } from '@angular/common/http';
import { signal } from '@angular/core';
import { fakeAsync, TestBed, tick } from '@angular/core/testing';
import { of } from 'rxjs';
import { AuthService } from '../login/auth.service';

import { PrefsService } from './prefs.service';

describe('PrefsService', () => {
  const prefsResponse = {
    userPrefs: { theme: 'dark' },
    sysPrefs: { locale: 'en' },
  };
  let authState = signal(false);
  let httpSpy: jasmine.SpyObj<HttpClient>;

  beforeEach(() => {
    authState = signal(false);
    httpSpy = jasmine.createSpyObj<HttpClient>('HttpClient', ['get', 'put']);
    httpSpy.get.and.returnValue(of(prefsResponse));
    httpSpy.put.and.returnValue(of({}));

    TestBed.configureTestingModule({
      providers: [
        PrefsService,
        { provide: HttpClient, useValue: httpSpy },
        {
          provide: AuthService,
          useValue: {
            isAuth: () => authState(),
          },
        },
      ],
    });
  });

  it('should be created', () => {
    const service = TestBed.inject(PrefsService);
    expect(service).toBeTruthy();
  });

  it('should not fetch preferences while unauthenticated', () => {
    TestBed.inject(PrefsService);
    expect(httpSpy.get).not.toHaveBeenCalled();
  });

  it('should fetch preferences when authenticated', () => {
    authState.set(true);
    TestBed.inject(PrefsService);
    expect(httpSpy.get).toHaveBeenCalledOnceWith('/api/prefs/stillbox');
  });

  it('should fetch preferences after auth transitions to authenticated', fakeAsync(() => {
    TestBed.inject(PrefsService);
    expect(httpSpy.get).not.toHaveBeenCalled();

    authState.set(true);
    tick();

    expect(httpSpy.get).toHaveBeenCalledOnceWith('/api/prefs/stillbox');
  }));

  it('should not persist preferences while unauthenticated', () => {
    const service = TestBed.inject(PrefsService);

    service.set('theme', 'dark');

    expect(httpSpy.put).not.toHaveBeenCalled();
  });
});
