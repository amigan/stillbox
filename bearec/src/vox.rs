use std::{
    sync::{Arc, Condvar, Mutex},
    thread::{self},
    time::Instant,
};

use crate::Opt;

use anyhow::Result;
use cpal::{
    traits::{DeviceTrait, StreamTrait},
    Device, Error, ErrorKind, FromSample, InputCallbackInfo, Sample, SampleFormat, SizedSample,
    SupportedBufferSize, SupportedStreamConfig,
};

use ringbuf::{
    traits::{Consumer, Producer, Split},
    HeapRb,
};

pub fn analyze_audio<T: RMSval + SizedSample + Send + hound::Sample + 'static>(
    opt: Opt,
    device: Device,
    orig_config: SupportedStreamConfig,
) -> Result<(), anyhow::Error>
where
    u8: FromSample<T>,
{
    let buffer_size = (orig_config.sample_rate() as u32 * opt.buffer_size_ms as u32) as u32 / 1000;
    let mut config = orig_config.config();
    match orig_config.buffer_size() {
        SupportedBufferSize::Range { min, max } => {
            if buffer_size < *min || buffer_size > *max {
                panic!("buffer size out of range {min}:{max} for audio device!")
            }
            config.buffer_size = cpal::BufferSize::Fixed(buffer_size);
        }
        SupportedBufferSize::Unknown => {}
    }
    let ring = HeapRb::<T>::new(buffer_size as usize);
    let (mut producer, mut consumer) = ring.split();
    let pair = Arc::new((Mutex::new(false), Condvar::new()));
    let pair_clone = Arc::clone(&pair);

    let consumer_hnd = thread::spawn(move || {
        println!("Starting thread, buffer size {buffer_size}");
        let mut buf: Vec<T> = vec![Sample::EQUILIBRIUM; buffer_size as usize];
        let mut call_buf: Vec<T> = Vec::new();
        let mut activation_time: Option<Instant> = None;
        let (lock, cvar) = &*pair;
        loop {
            let mut started = lock.lock().unwrap();

            while !*started {
                started = cvar.wait(started).unwrap();
            }
            *started = false;
            let read = consumer.pop_slice(&mut buf);

            if read == 0 {
                println!("read 0!");
                continue;
            }

            if read < buf.len() {
                buf[read..].fill(Sample::EQUILIBRIUM);
                eprintln!("buffer underrun {read}/{}", buf.len());
            }
            let rms_db = RMSable::db_rms(buf.as_slice());
            if rms_db as f64 > opt.activation_threshold_db {
                if activation_time == None {
                    // beginning of call. check radio squelch?
                    println!("activate {rms_db} dB");
                };
                activation_time = Some(Instant::now());
            };

            let now = Instant::now();
            if let Some(atime) = activation_time {
                call_buf.append(&mut buf.clone());
                if now.duration_since(atime).as_millis() >= opt.activation_fadeout_ms as u128 {
                    // call has ended. do something. maybe check squelch and hold open if necessary.
                    println!("call end {}s", now.duration_since(atime).as_secs_f32());
                    activation_time = None;
                    write_wav::<T>(&call_buf, &orig_config);
                    call_buf.truncate(0);
                };
            };
        }
    });

    let ring_fill_fn = move |data: &[T], _: &InputCallbackInfo| {
        let (lock, cvar) = &*pair_clone;

        let push_len = producer.push_slice(data);
        if push_len < data.len() {
            eprintln!("buffer overrun! {}", push_len);
        }
        let mut started = lock.lock().unwrap();
        *started = true;
        cvar.notify_one();
    };

    // A flag to indicate that recording is in progress.
    println!("Begin recording...");

    let err_fn = move |err: Error| match err.kind() {
        ErrorKind::DeviceChanged | ErrorKind::Xrun | ErrorKind::RealtimeDenied => {
            eprintln!("{err}")
        }
        _ => eprintln!("Stream error: {err}"),
    };

    let input_stream = device.build_input_stream(config.into(), ring_fill_fn, err_fn, None)?;
    input_stream.play()?;

    consumer_hnd.join().unwrap();
    drop(input_stream);

    Ok(())
}

fn sample_format(format: SampleFormat) -> hound::SampleFormat {
    if format.is_dsd() {
        panic!("DSD formats cannot be written to WAV files");
    } else if format.is_float() {
        hound::SampleFormat::Float
    } else {
        hound::SampleFormat::Int
    }
}

fn write_wav<T>(input: &[T], config: &SupportedStreamConfig)
where
    T: Sample + hound::Sample + FromSample<T>,
{
    let writer = hound::WavWriter::create("wr.wav", wav_spec_from_config(config));
    if let Ok(mut wav) = writer {
        for &sample in input.iter() {
            match wav.write_sample(sample) {
                Ok(v) => v,
                Err(e) => panic!("write wav {e}"),
            }
        }
    }
}

fn wav_spec_from_config(config: &SupportedStreamConfig) -> hound::WavSpec {
    hound::WavSpec {
        channels: config.channels() as _,
        sample_rate: config.sample_rate() as _,
        bits_per_sample: (config.sample_format().sample_size() * 8) as _,
        sample_format: sample_format(config.sample_format()),
    }
}
trait RMSable {
    fn rms(&self) -> f64;
    fn sample_max(&self) -> u32;
    fn db_rms(&self) -> f64 {
        let sample_max: u32 = self.sample_max();
        let mut db: f64 = -96.0;
        let rms: f64 = self.rms();

        if rms > 0.0 {
            db = 20.0 * f64::log10(rms / sample_max as f64);
        }

        db
    }
}

pub trait RMSval {
    fn sample_val(&self) -> i32;
    fn sample_max(&self) -> u32;
}

impl RMSval for u8 {
    fn sample_val(&self) -> i32 {
        let samp = u8::from_sample(*self);
        (samp as i32) - 0x80_i32
    }

    fn sample_max(&self) -> u32 {
        0x7f
    }
}

impl RMSval for i8 {
    fn sample_val(&self) -> i32 {
        i8::from_sample(*self) as i32
    }

    fn sample_max(&self) -> u32 {
        0x7f
    }
}

impl RMSval for i16 {
    fn sample_val(&self) -> i32 {
        i16::from_le(*self) as i32
    }

    fn sample_max(&self) -> u32 {
        0x7fff
    }
}

impl<T: RMSval> RMSable for [T] {
    fn rms(&self) -> f64 {
        let mut sum2: u64 = 0;
        for b in self {
            let v = b.sample_val() as i32;
            sum2 += (v * v) as u64;
        }
        f64::sqrt(sum2 as f64 / self.len() as f64)
    }

    fn sample_max(&self) -> u32 {
        match self.first() {
            None => panic!("no samples"),
            Some(s) => s.sample_max(),
        }
    }
}
