use core::fmt;
use reqwest_websocket::{Upgrade, UpgradeResponse, WebSocket};
use crate::{client::RESTClient, nexus::NexusError::{URLError, WebSocketError}};

pub struct NexusClient {
    rest_client: RESTClient,
    ws: WebSocket,
}

#[derive(Debug)]
pub enum NexusError {
    HTTPError(reqwest::Error),
    WebSocketError(reqwest_websocket::Error),
    URLError(url::ParseError),
}

impl std::error::Error for NexusError {}
impl fmt::Display for NexusError {
    fn fmt(&self, f: &mut fmt::Formatter) -> fmt::Result {
        match self {
            NexusError::HTTPError(e) => write!(f, "HTTP Error: {}", e.to_string()),
            NexusError::URLError(e) => write!(f, "URL Error: {}", e.to_string()),
            NexusError::WebSocketError(e) => write!(f, "WS error: {}", e.to_string()),
        }
    }
}

mod upload;

pub async fn new_nexus_client(rest_client: RESTClient) -> Result<NexusClient, NexusError> {
    let ws_url = match rest_client.websocket_url() {
        Ok(u) => u,
        Err(err) => return Err(URLError(err)),
    };

    let resp = match rest_client.client.get(ws_url).upgrade().send().await {
        Ok(ur) => ur,
        Err(e) => return Err(NexusError::WebSocketError(e)),
    };

    let websocket = match resp.into_websocket().await {
        Ok(ws) => ws,
        Err(err) => return Err(NexusError::WebSocketError(err)),
    };

    Ok(NexusClient {
        rest_client,
        ws: websocket,
    })
}