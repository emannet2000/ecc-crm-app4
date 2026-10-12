port module Ports exposing (downloadFile, storeTheme, storeToken, recordChanged, draftStorage, storedDraft, workspaceAction, recordSaved)

import Json.Encode

{-| JavaScript interop.
-}


port storeToken : Maybe String -> Cmd msg


port downloadFile : { filename : String, content : String, mime : String } -> Cmd msg


port storeTheme : String -> Cmd msg


port recordChanged : (String -> msg) -> Sub msg


port draftStorage : { action : String, key : String, fields : Json.Encode.Value } -> Cmd msg


port storedDraft : (Json.Encode.Value -> msg) -> Sub msg


port workspaceAction : ({ entity : String, id : String, action : String } -> msg) -> Sub msg


port recordSaved : String -> Cmd msg
