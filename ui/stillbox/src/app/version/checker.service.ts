import { Injectable } from '@angular/core';
import { SwUpdate } from '@angular/service-worker';
import { Subscription } from 'rxjs';

@Injectable({
  providedIn: 'root',
})
export class CheckerService {
  updateAvailable = false;
  versionSub!: Subscription;

  doUpdate() {
    location.reload();
  }

  constructor(private swUpd: SwUpdate) {
    this.checkUpdate();
  }

  checkUpdate() {
    this.versionSub?.unsubscribe();
    if (!this.swUpd.isEnabled) {
      return;
    }
    this.versionSub = this.swUpd.versionUpdates.subscribe((evt) => {
      switch (evt.type) {
        case 'VERSION_DETECTED':
          console.log('Detected new version ${evt.version.hash}');
          break;
        case 'VERSION_READY':
          console.log(`Current app version: ${evt.currentVersion.hash}`);
          console.log(
            `New app version ready for use: ${evt.latestVersion.hash}`,
          );
          this.updateAvailable = true;
          break;
        case 'VERSION_INSTALLATION_FAILED':
          console.log(
            `Failed to install app version '${evt.version.hash}': ${evt.error}`,
          );
      }
    });
  }
}
