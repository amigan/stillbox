use clap::Parser;

mod audio;
mod vox;

#[derive(Parser, Debug)]
#[command(version, about = "CPAL record_wav example", long_about = None)]
struct Opt {
    /// The audio device to use.
    #[arg(short, long)]
    device: Option<String>,

    /// How long to record, in seconds
    #[arg(long, default_value_t = 3)]
    duration: u64,

    /// Use the JACK host. Requires `--features jack`.
    #[arg(long, default_value_t = false)]
    jack: bool,

    /// Use the PulseAudio host. Requires `--features pulseaudio`.
    #[arg(long, default_value_t = false)]
    pulseaudio: bool,

    /// Use the Pipewire host. Requires `--features pipewire`
    #[arg(long, default_value_t = false)]
    pipewire: bool,

    #[arg(long, default_value_t = -50.0)]
    activation_threshold_db: f64,

    #[arg(long, default_value_t = 500)]
    activation_fadeout_ms: i32,

    #[arg(long, default_value_t = 250)]
    buffer_size_ms: i32,
}

fn main() -> anyhow::Result<()> {
    let opt = Opt::parse();
    audio::capture(opt)
}
