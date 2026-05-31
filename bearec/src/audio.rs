use super::vox::analyze_audio;
use crate::Opt;
use anyhow::Result;
use cpal::{
    traits::{DeviceTrait, HostTrait},
    Error, ErrorKind, HostId,
    SampleFormat::{self, I16},
};

pub fn capture(opt: Opt) -> Result<(), anyhow::Error> {
    // Jack/PulseAudio support must be enabled at compile time, and is
    // only available on some platforms.
    #[allow(unused_mut, unused_assignments)]
    let mut jack_host_id: Result<HostId, Error> = Err(ErrorKind::HostUnavailable.into());
    #[allow(unused_mut, unused_assignments)]
    let mut pulseaudio_host_id: Result<HostId, Error> = Err(ErrorKind::HostUnavailable.into());
    #[allow(unused_mut, unused_assignments)]
    let mut pipewire_host_id: Result<HostId, Error> = Err(ErrorKind::HostUnavailable.into());
    #[cfg(any(
        target_os = "linux",
        target_os = "dragonfly",
        target_os = "freebsd",
        target_os = "netbsd"
    ))]
    {
        #[cfg(feature = "jack")]
        {
            jack_host_id = Ok(HostId::Jack);
        }

        #[cfg(feature = "pulseaudio")]
        {
            pulseaudio_host_id = Ok(HostId::PulseAudio);
        }
        #[cfg(feature = "pipewire")]
        {
            pipewire_host_id = Ok(HostId::PipeWire);
        }
    }

    // Manually check for flags. Can be passed through cargo with -- e.g.
    // cargo run --release --example record_wav --features jack -- --jack
    let host = if opt.jack {
        jack_host_id
            .and_then(cpal::host_from_id)
            .expect("make sure `--features jack` is specified, and the platform is supported")
    } else if opt.pulseaudio {
        pulseaudio_host_id
            .and_then(cpal::host_from_id)
            .expect("make sure `--features pulseaudio` is specified, and the platform is supported")
    } else if opt.pipewire {
        pipewire_host_id
            .and_then(cpal::host_from_id)
            .expect("make sure `--features pipewire` is specified, and the platform is supported")
    } else {
        cpal::default_host()
    };

    // Set up the input device and stream with the default input config.
    let device = if let Some(device) = &opt.device {
        let id = &device.parse().expect("failed to parse input device id");
        host.device_by_id(id)
    } else {
        host.default_input_device()
    }
    .expect("failed to find input device");

    println!("Input device: {}", device.id()?);

    if !device.supports_input() {
        panic!("device does not support input");
    }

    let configs_range = device
        .supported_input_configs()
        .expect("error querying configs");

    let config = configs_range
        .filter(|c| c.channels() == 1 && c.sample_format() == I16)
        .next()
        .expect("no supported input configs")
        .with_sample_rate(16000);

    let sample_format = config.sample_format();
    println!("Default input/output config: {config:?}");

    match sample_format {
        SampleFormat::I16 => analyze_audio::<i16>(opt, device, config),
        SampleFormat::I8 => analyze_audio::<i8>(opt, device, config),
        sample_format => panic!("unsupported sample format {sample_format}"),
    }
}
