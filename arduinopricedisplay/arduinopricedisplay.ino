/*
 * This is just minimal version for ESP8266 and TM1637 7-segment display
 * Different thing than e-paper version
 * This is total hack, just my personal use.
 */
#include <NTPClient.h>
#include <ESP8266WiFi.h>
#include <WiFiUdp.h>
#include "ESP8266HTTPClient.h"
#include <TM1637TinyDisplay.h>
#include <time.h>

#include <ArduinoJson.h>

/* Digital Pins to TM1637 */
#define CLK 2
#define DIO 0

TM1637TinyDisplay display(CLK, DIO);
uint8_t dots = 0b01000000;

#include "wlanssidandpass.h" //Please provide your own wlan ssid and password on header file 
const char *ssid     = WLANSSID;
const char *password = WLANPASSWORD;


const uint8_t fingerprint[20]={0x5B,0x94,0x13,0x7B,0x61,0xD9,0xB5,0x0A,0x79,0x32,0x67,0xF5,0x7D,0xDA,0xAD,0xDC,0x55,0x7C,0x7D,0x55};

WiFiUDP ntpUDP;

NTPClient timeClient(ntpUDP,"fi.pool.ntp.org",3600, 3*60*60*1000);

WiFiClient client;
HTTPClient https;


const char * headerKeys[] = {"date", "server"} ;
const size_t numberOfHeaders = 2;

void setup(){
  Serial.begin(115200);
  while (!Serial) {
    ; // wait for serial port to connect. Needed for native USB
  }
  WiFi.begin(ssid, password);

  while ( WiFi.status() != WL_CONNECTED ) {
    delay ( 500 );
    Serial.print ( "." );
  }

  timeClient.begin();

  display.setBrightness(BRIGHT_HIGH);
  display.clear();
  checktime();
  checkprices();
}



#define STARTYEAR 2022
#define NYEARS 30
const time_t summerstart[]={1648342800,1679792400,1711846800,1743296400,1774746000,1806195600,1837645200,1869094800,1901149200,1932598800,1964048400,1995498000,2026947600,2058397200,2090451600,2121901200,2153350800,2184800400,2216250000,2248304400,2279754000,2311203600,2342653200,2374102800,2405552400,2437606800,2469056400,2500506000,2531955600,2563405200};
const time_t summerend[]={1667091599,1698541199,1729990799,1761440399,1792889999,1824944399,1856393999,1887843599,1919293199,1950742799,1982797199,2014246799,2045696399,2077145999,2108595599,2140045199,2172099599,2203549199,2234998799,2266448399,2297897999,2329347599,2361401999,2392851599,2424301199,2455750799,2487200399,2519254799,2550704399,2582153999};

//When hours roll from 23 to 0, get new prices
int prevHour;

bool checktime(){
  timeClient.update();
  time_t rawtime = timeClient.getEpochTime();
  struct tm  ts;
  ts = *localtime(&rawtime);  
  time_t sStart=summerstart[ts.tm_year+1900-STARTYEAR];
  time_t sEnd=summerend[ts.tm_year+1900-STARTYEAR];
  Serial.println("kesa");
  Serial.println(sStart);
  Serial.println(sEnd);

  if ((sStart<rawtime)&&(rawtime<sEnd)){
      Serial.println("KESÄAIKA NYT");
    timeClient.setTimeOffset(3*60*60);
  }else{
      Serial.println("TALVIAIKA NYT");
    timeClient.setTimeOffset(2*60*60);    
  }

  if ((prevHour==23) && (ts.tm_hour==0)){
    prevHour=ts.tm_hour;
    return true;    
  }
  prevHour=ts.tm_hour;
  return false;
}


char vattenfallurl[80];
float hourprices[24];

void checkprices(){
  timeClient.update();
  time_t rawtime = timeClient.getEpochTime();
  struct tm  ts;
  ts = *localtime(&rawtime);  

  std::unique_ptr<BearSSL::WiFiClientSecure>client(new BearSSL::WiFiClientSecure);
  //client->setInsecure(); //BAD, REALLY BAD
  client->setFingerprint(fingerprint);

  sprintf(vattenfallurl,"https://www.vattenfall.fi/api/price/spot/%d-%02d-%02d/%d-%02d-%02d?lang=fi",
    ts.tm_year+1900,ts.tm_mon+1,ts.tm_mday,
    ts.tm_year+1900,ts.tm_mon+1,ts.tm_mday);

  Serial.println("GET ");
  Serial.print(vattenfallurl);
  Serial.println("");

  String url=vattenfallurl;
  //https.connect(vattenfallurl,443);
  https.begin(*client,url.c_str());
  //http.collectHeaders(headerKeys, numberOfHeaders);
  int httpCode = https.GET();
  Serial.println(httpCode);
  if (httpCode > 0) {
    String raw=https.getString();
    Serial.println("RAW IS");
    Serial.println(raw);
    //Parsitaan \"value\"
      int index=0;
    String pricestring;
    for(int hour=0;hour<24;hour++){
      index=raw.indexOf("\"value\":");
      raw=raw.substring(index+8);
      pricestring=raw.substring(0,raw.indexOf(","));
      hourprices[hour]=pricestring.toFloat();
    }
    Serial.println("HOUR PRICES: ");
    for(int hour=0;hour<24;hour++){
      Serial.print(hourprices[hour]);
      Serial.print(" ");
    }
  }else{
    Serial.printf("[HTTP] GET... failed, error: %s\n", https.errorToString(httpCode).c_str());
  }
}


void loop() {
  if (checktime()){
    checkprices();
  }
  //Show price
  Serial.println("PRICE");
  float priceNow=hourprices[timeClient.getHours()];
  Serial.print(priceNow);
  Serial.print("snt/kWh at ");
  Serial.println(timeClient.getHours());
  display.clear();
  display.showNumberDec(round(priceNow*1.24), 0, false, 4, 0); //HACK  24% tax added

  delay(3000);
  Serial.println("TIME");
  Serial.println(timeClient.getFormattedTime());
  display.showNumberDec(timeClient.getHours(), dots, true, 2, 0);
  display.showNumberDec(timeClient.getMinutes(), dots, true, 2, 2);

  delay(5000);  
}
