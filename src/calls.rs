use actix::prelude::{Actor, Context, Handler, Recipient};
use std::collections::{HashMap, HashSet};
use std::sync::Arc;
use std::time::SystemTime;
use prost::Message as Message;
use prost_types::Timestamp;

pub mod stillbox {
    pub mod calls {
        include!(concat!(env!("OUT_DIR"), "/stillbox.calls.rs"));
    }
}

pub fn create_call() -> stillbox::calls::Call {
    let mut call = stillbox::calls::Call::default();
    call.metadata = Some(stillbox::calls::CallMetadata {
        id: String::from("asd"),
        timestamp: Some(Timestamp::from(SystemTime::now())),
        talkgroup: 123,
        filename: String::from("somefile"),
        mime_type: String::from("audio/mp3"),
        frequencies: Vec::new(),
        frequency: 123455,
        patches: Vec::new(),
        source: 0,
        sources: Vec::new(),
        system: 0x70,
    });

    call
}

type Socket = Recipient<Vec<u8>>;

type SessionId = uuid::Uuid;

//use stillbox::calls;

pub struct StillBox {
    listeners: HashMap<SessionId, Socket>,
}

impl Default for StillBox {
    fn default() -> StillBox {
        StillBox {
            listeners: HashMap::new(),
        }
    }
}

impl StillBox {
}
