# Unifi Access Hubitat Middleware (UAHM)

A Go application that enables Hubitat hub integration with UniFi Access Controller (UAC). 
This middleware listens for webhooks from UAC and Hubitat, allowing you to control and monitor UAC doors from Hubitat.

## Features

- Provides a Lock, Contact Sensor, and Switch device type in Hubitat for each UAC door
- Listens for webhooks from UniFi Access and Hubitat. Polling is exclusively used on UAC where webhooks are not currently available (e.g. for door rule status).
- Supports multiple UAC doors, each mapped to Hubitat virtual devices
- Secure communication using a configurable auth token

## Configuration

### Hubitat

#### Create Virtual Devices
1. Go to **Devices** > **Add device** > **Virtual**.
    - Create a **Virtual Lock**, **Virtual Contact**, and **Virtual Switch** for each UAC door.
    - For the **Virtual Switch**, go to **Preferences** and enable auto off with a 10 second delay.

#### Enable Maker API
1. Go to **Apps** > **Add Built-In App** > **Maker API**.
2. Configure the app (only enable the fields listed below, all other fields should be disabled/unchecked):
    -  **Maker API Label**: "Maker API (UAHM)" (or any name you prefer)
    -  **Security**: Enable "Allow Access via Local IP Address"
    - **Devices**: Select the virtual devices that were created in the prior steps for each UAC door (Lock, Contact Sensor, Switch)
    - Set "URL to POST device events to" `http://your-server-ip:9423/webhook/hubitat?authorization=your_auth_token` (replace `your-server-ip` and `your_auth_token` with your actual server IP and auth token).
3. Press the **Get All Devices** hyperlink and note the device IDs for each virtual device. You will need these IDs for the configuration file.
    - Note the URL, specifically the `apps/api/123` part, as you will need it for the configuration file and the `access_token` for the Maker API.

### UniFi Access Controller (UAC)
#### Enable API Access
1. Log in to your UniFi Access Controller.
2. Go to **Settings** > **General** > **API Token**.
3. Enable the API and generate an API key.
   - Name: "unifi-access-hubitat-middleware" (or any name you prefer)
   - Permissions: Enable "Edit" for "Locations" and "Webhooks". All other permissions can be left as "None".
4. Note the API Key and URL, which should look like `https://your-uac-ip:12445`.


#### Nginx Proxy (Optional)
Recommend using Nginx to terminate SSL (e.g Nginx Proxy Manager) and forward requests to the app. 
This is optional but further secures your setup. Instructions to set this up is beyond the scope of this README.

#### UAHM Configuration

Create a `config.yaml` file in the project root.

```yaml
server:
  base_url: "http://your-server-url"
  auth_token: "your_auth_token"

uac:
  base_url: "https://your-uac-url:12445"
  api_key: "your_uac_api_key"

hubitat:
  base_url: "http://your-hubitat-url/apps/api/123"
  access_token: "your_hubitat_access_token"

doors:
  - uac_id: "uac-door-id-1"
    hubitat_contact_id: "contact-device-id"
    hubitat_lock_id: "lock-device-id"
    hubitat_switch_id: "switch-device-id"
  # Add more doors as needed
```

**Fields:**
- `server.base_url`: URL where this app is accessible
- `server.auth_token`: Random token of your choice for securing webhooks
- `uac.base_url` / `uac.api_key`: UniFi Access Controller API details
- `hubitat.base_url` / `hubitat.access_token`: Hubitat Maker API details
- `doors`: Map UAC door IDs to Hubitat device IDs


## Running with Docker Compose

Build and run the app using Docker Compose:

```sh
docker compose up -d
```

- The app listens on port `9423` by default.


