# AudioMuse-AI-MusicServer
Prototype music server built on the Open Subsonic API, designed to showcase AudioMuse-AI’s advanced sonic analysis capabilities.

# Developer
This is instruction to run both backend and frontned on your developing envirorment

## Compile and run backend
Going in /music-server-backend/

```
go mod init music-server-backend
go mod tidy
go build -o music-server
./music-server
```

API will be reacheable on http://localhost:8080/rest/

API actually exposed:
```
[GIN-debug] GET    /rest/ping.view           --> main.subsonicPing (3 handlers)
[GIN-debug] GET    /rest/getLicense.view     --> main.subsonicGetLicense (3 handlers)
[GIN-debug] GET    /rest/stream.view         --> main.subsonicStream (3 handlers)
[GIN-debug] GET    /rest/getArtists.view     --> main.subsonicGetArtists (3 handlers)
[GIN-debug] GET    /rest/getAlbumList2.view  --> main.subsonicGetAlbumList2 (3 handlers)
[GIN-debug] GET    /rest/getPlaylists.view   --> main.subsonicGetPlaylists (3 handlers)
[GIN-debug] GET    /rest/getPlaylist.view    --> main.subsonicGetPlaylist (3 handlers)
[GIN-debug] GET    /rest/createPlaylist.view --> main.subsonicCreatePlaylist (3 handlers)
[GIN-debug] POST   /rest/createPlaylist.view --> main.subsonicCreatePlaylist (3 handlers)
[GIN-debug] PUT    /rest/createPlaylist.view --> main.subsonicCreatePlaylist (3 handlers)
[GIN-debug] PATCH  /rest/createPlaylist.view --> main.subsonicCreatePlaylist (3 handlers)
[GIN-debug] HEAD   /rest/createPlaylist.view --> main.subsonicCreatePlaylist (3 handlers)
[GIN-debug] OPTIONS /rest/createPlaylist.view --> main.subsonicCreatePlaylist (3 handlers)
[GIN-debug] DELETE /rest/createPlaylist.view --> main.subsonicCreatePlaylist (3 handlers)
[GIN-debug] CONNECT /rest/createPlaylist.view --> main.subsonicCreatePlaylist (3 handlers)
[GIN-debug] TRACE  /rest/createPlaylist.view --> main.subsonicCreatePlaylist (3 handlers)
[GIN-debug] GET    /rest/updatePlaylist.view --> main.subsonicUpdatePlaylist (3 handlers)
[GIN-debug] GET    /rest/deletePlaylist.view --> main.subsonicDeletePlaylist (3 handlers)
[GIN-debug] GET    /rest/getAlbum.view       --> main.subsonicGetAlbum (3 handlers)
[GIN-debug] GET    /rest/search2.view        --> main.subsonicSearch2 (3 handlers)
[GIN-debug] GET    /rest/search3.view        --> main.subsonicSearch2 (3 handlers)
[GIN-debug] GET    /rest/getRandomSongs.view --> main.subsonicGetRandomSongs (3 handlers)
[GIN-debug] GET    /rest/getCoverArt.view    --> main.subsonicGetCoverArt (3 handlers)
[GIN-debug] GET    /rest/tokenInfo.view      --> main.subsonicTokenInfo (3 handlers)
[GIN-debug] GET    /rest/startScan.view      --> main.subsonicStartScan (3 handlers)
[GIN-debug] POST   /rest/startScan.view      --> main.subsonicStartScan (3 handlers)
[GIN-debug] PUT    /rest/startScan.view      --> main.subsonicStartScan (3 handlers)
[GIN-debug] PATCH  /rest/startScan.view      --> main.subsonicStartScan (3 handlers)
[GIN-debug] HEAD   /rest/startScan.view      --> main.subsonicStartScan (3 handlers)
[GIN-debug] OPTIONS /rest/startScan.view      --> main.subsonicStartScan (3 handlers)
[GIN-debug] DELETE /rest/startScan.view      --> main.subsonicStartScan (3 handlers)
[GIN-debug] CONNECT /rest/startScan.view      --> main.subsonicStartScan (3 handlers)
[GIN-debug] TRACE  /rest/startScan.view      --> main.subsonicStartScan (3 handlers)
[GIN-debug] GET    /rest/getScanStatus.view  --> main.subsonicGetScanStatus (3 handlers)
[GIN-debug] GET    /rest/getUsers.view       --> main.subsonicGetUsers (3 handlers)
[GIN-debug] GET    /rest/createUser.view     --> main.subsonicCreateUser (3 handlers)
[GIN-debug] GET    /rest/updateUser.view     --> main.subsonicUpdateUser (3 handlers)
[GIN-debug] GET    /rest/deleteUser.view     --> main.subsonicDeleteUser (3 handlers)
[GIN-debug] GET    /rest/changePassword.view --> main.subsonicChangePassword (3 handlers)
[GIN-debug] GET    /rest/getConfiguration.view --> main.subsonicGetConfiguration (3 handlers)
[GIN-debug] GET    /rest/setConfiguration.view --> main.subsonicSetConfiguration (3 handlers)
[GIN-debug] GET    /rest/getSimilarSongs.view --> main.subsonicGetSimilarSongs (3 handlers)
[GIN-debug] GET    /rest/getSongPath.view    --> main.subsonicGetSongPath (3 handlers)
[GIN-debug] POST   /api/v1/user/login        --> main.loginUser (3 handlers)
[GIN-debug] GET    /api/v1/admin/browse      --> main.browseFiles (5 handlers)
[GIN-debug] POST   /api/v1/admin/scan/cancel --> main.cancelAdminScan (5 handlers)
```

## Compile and run frontend
Goin ing /music-server-frontend/

```
npm install
npm start
```

Frontend will be reacheable on http://localhost:3000/ you can do the first login with admin/admin
