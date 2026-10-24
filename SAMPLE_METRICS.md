# Sample metrics

Output of `/metrics` from a MiFi X PRO 5G (M3000) running firmware
`THN-1.53-5.4-2.594.1.2-28.181.4-28.181.4`, captured on 2026-10-23 with the
default configuration.

Anonymized values:

- WAN addresses in `inseego_wan_info` were replaced with documentation ranges
  (`192.0.2.0/24`, `2001:db8::/32`).
- The carrier name in `inseego_network_info` was replaced with `CARRIER`.

The standard `go_*` and `process_*` metrics are omitted.

## Live capture

At capture time the GPS receiver was still searching for satellites
(`inseego_gps_receiver_info{status="Searching"}`, `inseego_gps_fix_valid 0`),
so position metrics are absent. The stream was still sending sentences, so
`inseego_gps_up` is `1`. The modem was on 5G non-standalone, with an LTE
anchor band and an NR band in `inseego_network_info`.

```text
# HELP inseego_airplane_mode 1 if airplane mode is on.
# TYPE inseego_airplane_mode gauge
inseego_airplane_mode 0
# HELP inseego_battery_charging 1 if the battery is charging.
# TYPE inseego_battery_charging gauge
inseego_battery_charging 0
# HELP inseego_battery_charging_source_info Current charging source, always 1.
# TYPE inseego_battery_charging_source_info gauge
inseego_battery_charging_source_info{source="ChargingSourceNone"} 1
# HELP inseego_battery_percent Battery charge level in percent.
# TYPE inseego_battery_percent gauge
inseego_battery_percent 75
# HELP inseego_battery_present 1 if a battery is detected.
# TYPE inseego_battery_present gauge
inseego_battery_present 1
# HELP inseego_clients Number of connected clients, by type (total, primary, guest, wifi).
# TYPE inseego_clients gauge
inseego_clients{type="guest"} 0
inseego_clients{type="primary"} 1
inseego_clients{type="total"} 1
inseego_clients{type="wifi"} 1
# HELP inseego_clients_max Maximum number of clients supported, by type (total, wifi).
# TYPE inseego_clients_max gauge
inseego_clients_max{type="total"} 34
inseego_clients_max{type="wifi"} 32
# HELP inseego_connected 1 if the cellular data connection state is Connected.
# TYPE inseego_connected gauge
inseego_connected 1
# HELP inseego_connection_duration_seconds Duration of the current cellular data connection.
# TYPE inseego_connection_duration_seconds gauge
inseego_connection_duration_seconds 923
# HELP inseego_ethernet_enabled 1 if the Ethernet port is enabled.
# TYPE inseego_ethernet_enabled gauge
inseego_ethernet_enabled 1
# HELP inseego_external_antenna_info External antenna status, always 1.
# TYPE inseego_external_antenna_info gauge
inseego_external_antenna_info{status="not_connected"} 1
# HELP inseego_firmware_info Installed firmware version, always 1.
# TYPE inseego_firmware_info gauge
inseego_firmware_info{version="THN-1.53-5.4-2.594.1.2-28.181.4-28.181.4"} 1
# HELP inseego_gps_fetch_duration_seconds Duration of the last fetch from the NMEA stream.
# TYPE inseego_gps_fetch_duration_seconds gauge
inseego_gps_fetch_duration_seconds 0.2603325
# HELP inseego_gps_fetch_failures_total Fetches that failed to connect or received no valid sentences.
# TYPE inseego_gps_fetch_failures_total counter
inseego_gps_fetch_failures_total 0
# HELP inseego_gps_fetches_total Fetches from the NMEA stream.
# TYPE inseego_gps_fetches_total counter
inseego_gps_fetches_total 1
# HELP inseego_gps_fix_quality GGA fix quality (0=invalid, 1=GPS, 2=DGPS, ...).
# TYPE inseego_gps_fix_quality gauge
inseego_gps_fix_quality 0
# HELP inseego_gps_fix_valid 1 if the last RMC sentence reported a valid fix (status A).
# TYPE inseego_gps_fix_valid gauge
inseego_gps_fix_valid 0
# HELP inseego_gps_last_sentence_timestamp_seconds Unix time at which the last valid NMEA sentence was received.
# TYPE inseego_gps_last_sentence_timestamp_seconds gauge
inseego_gps_last_sentence_timestamp_seconds 1.792770629022194e+09
# HELP inseego_gps_mode_info FAA mode indicator of the last RMC sentence, always 1.
# TYPE inseego_gps_mode_info gauge
inseego_gps_mode_info{mode="N"} 1
# HELP inseego_gps_receiver_info GPS receiver status as reported by the device, always 1.
# TYPE inseego_gps_receiver_info gauge
inseego_gps_receiver_info{status="Searching"} 1
# HELP inseego_gps_satellites_used Number of satellites used in the fix (GGA).
# TYPE inseego_gps_satellites_used gauge
inseego_gps_satellites_used 0
# HELP inseego_gps_sentences_total Valid NMEA sentences received, by talker+type.
# TYPE inseego_gps_sentences_total counter
inseego_gps_sentences_total{sentence="GPGGA"} 1
inseego_gps_sentences_total{sentence="GPGSV"} 2
inseego_gps_sentences_total{sentence="GPRMC"} 1
inseego_gps_sentences_total{sentence="GPVTG"} 1
# HELP inseego_gps_up 1 if the last fetch from the MiFi NMEA stream received valid sentences.
# TYPE inseego_gps_up gauge
inseego_gps_up 1
# HELP inseego_guest_wifi_enabled 1 if the guest Wi-Fi network is enabled.
# TYPE inseego_guest_wifi_enabled gauge
inseego_guest_wifi_enabled 1
# HELP inseego_mobile_data_enabled 1 if mobile data is enabled.
# TYPE inseego_mobile_data_enabled gauge
inseego_mobile_data_enabled 1
# HELP inseego_network_info Cellular network details, always 1.
# TYPE inseego_network_info gauge
inseego_network_info{band="B12, n78",bandwidth="30 MHz, 45 MHz",network="CARRIER_NAME",pci="531",technology="5G"} 1
# HELP inseego_received_bytes_total Bytes received over the cellular connection, as reported by the device.
# TYPE inseego_received_bytes_total counter
inseego_received_bytes_total 6.747829e+06
# HELP inseego_roaming 1 if the device is roaming.
# TYPE inseego_roaming gauge
inseego_roaming 0
# HELP inseego_signal_bars Signal strength in bars, as shown on the device.
# TYPE inseego_signal_bars gauge
inseego_signal_bars 3
# HELP inseego_signal_snr_db Signal-to-noise ratio in dB.
# TYPE inseego_signal_snr_db gauge
inseego_signal_snr_db 4
# HELP inseego_sim_info SIM card status, always 1.
# TYPE inseego_sim_info gauge
inseego_sim_info{status="Ready"} 1
# HELP inseego_sms_unread Number of unread SMS messages.
# TYPE inseego_sms_unread gauge
inseego_sms_unread 0
# HELP inseego_status_fetch_duration_seconds Time taken by the last fetch of the /srv/status endpoint.
# TYPE inseego_status_fetch_duration_seconds gauge
inseego_status_fetch_duration_seconds 0.292518084
# HELP inseego_status_last_fetch_timestamp_seconds Unix time of the last successful fetch of the /srv/status endpoint.
# TYPE inseego_status_last_fetch_timestamp_seconds gauge
inseego_status_last_fetch_timestamp_seconds 1.7927706290545988e+09
# HELP inseego_status_up 1 if the last fetch of the /srv/status endpoint succeeded.
# TYPE inseego_status_up gauge
inseego_status_up 1
# HELP inseego_transmitted_bytes_total Bytes transmitted over the cellular connection, as reported by the device.
# TYPE inseego_transmitted_bytes_total counter
inseego_transmitted_bytes_total 4.091899e+06
# HELP inseego_vpn_info VPN connection state, always 1.
# TYPE inseego_vpn_info gauge
inseego_vpn_info{state="Not configured"} 1
# HELP inseego_wan_info WAN addressing, always 1.
# TYPE inseego_wan_info gauge
inseego_wan_info{gateway="192.0.2.1",ipv4_address="192.0.2.2",ipv6_address="2001:db8::2"} 1
# HELP inseego_wifi_enabled 1 if Wi-Fi is enabled.
# TYPE inseego_wifi_enabled gauge
inseego_wifi_enabled 1
```

## GPS metrics with a valid fix

This is an example, not part of the capture above. It shows the extra
`inseego_gps_*` series exported once the receiver has a fix. The values come
from an earlier live run, except the coordinates, which are placeholders.

```text
# HELP inseego_gps_fix_valid 1 if the last RMC sentence reported a valid fix (status A).
# TYPE inseego_gps_fix_valid gauge
inseego_gps_fix_valid 1
# HELP inseego_gps_fix_quality GGA fix quality (0=invalid, 1=GPS, 2=DGPS, ...).
# TYPE inseego_gps_fix_quality gauge
inseego_gps_fix_quality 1
# HELP inseego_gps_fix_timestamp_seconds UTC time of the last RMC fix as a Unix timestamp.
# TYPE inseego_gps_fix_timestamp_seconds gauge
inseego_gps_fix_timestamp_seconds 1.792709751e+09
# HELP inseego_gps_latitude_degrees Latitude of the last fix in decimal degrees (negative is south).
# TYPE inseego_gps_latitude_degrees gauge
inseego_gps_latitude_degrees 12.345678
# HELP inseego_gps_longitude_degrees Longitude of the last fix in decimal degrees (negative is west).
# TYPE inseego_gps_longitude_degrees gauge
inseego_gps_longitude_degrees -98.765432
# HELP inseego_gps_altitude_meters Altitude above mean sea level in meters (GGA).
# TYPE inseego_gps_altitude_meters gauge
inseego_gps_altitude_meters 31.4
# HELP inseego_gps_geoid_separation_meters Geoid separation in meters (GGA).
# TYPE inseego_gps_geoid_separation_meters gauge
inseego_gps_geoid_separation_meters -32.7
# HELP inseego_gps_hdop Horizontal dilution of precision (GGA).
# TYPE inseego_gps_hdop gauge
inseego_gps_hdop 0.7
# HELP inseego_gps_satellites_used Number of satellites used in the fix (GGA).
# TYPE inseego_gps_satellites_used gauge
inseego_gps_satellites_used 7
# HELP inseego_gps_speed_knots Speed over ground in knots.
# TYPE inseego_gps_speed_knots gauge
inseego_gps_speed_knots 0
# HELP inseego_gps_magnetic_variation_degrees Magnetic variation in degrees (negative is west).
# TYPE inseego_gps_magnetic_variation_degrees gauge
inseego_gps_magnetic_variation_degrees -13.4
# HELP inseego_gps_mode_info FAA mode indicator of the last RMC sentence, always 1.
# TYPE inseego_gps_mode_info gauge
inseego_gps_mode_info{mode="A"} 1
# HELP inseego_gps_receiver_info GPS receiver status as reported by the device, always 1.
# TYPE inseego_gps_receiver_info gauge
inseego_gps_receiver_info{status="GotFix"} 1
```
