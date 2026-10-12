module View.Filters exposing (distinctOwners, isStale, passesFilters)

{-| Deal filtering helpers.
-}

import Html exposing (..)
import Types exposing (..)
import View.Format exposing (dateToDays)


isStale : Maybe Int -> Bool
isStale maybeDays =
    case maybeDays of
        Just d ->
            d >= 45

        Nothing ->
            False


distinctOwners : List Deal -> List String
distinctOwners deals =
    deals
        |> List.map .owner
        |> List.filter (\o -> not (String.isEmpty o))
        |> List.foldl
            (\o acc ->
                if List.member o acc then
                    acc

                else
                    acc ++ [ o ]
            )
            []
        |> List.sort


passesFilters : Model -> Deal -> Bool
passesFilters model d =
    let
        ownerOk =
            if String.isEmpty model.ownerFilter then
                True

            else if model.ownerFilter == "__mine__" then
                case model.user of
                    Just user ->
                        d.owner == user.id || d.owner == user.name || ((String.isEmpty d.owner || String.toLower (String.trim d.owner) == "unassigned") && d.ownerId == user.id)

                    Nothing ->
                        False

            else
                d.owner == model.ownerFilter

        closeDateOk =
            if String.isEmpty model.dateFromFilter && String.isEmpty model.dateToFilter then
                True

            else
                case dateToDays d.closeDate of
                    Nothing ->
                        False

                    Just cd ->
                        let
                            fromOk =
                                if String.isEmpty model.dateFromFilter then
                                    True

                                else
                                    case dateToDays model.dateFromFilter of
                                        Just fd ->
                                            cd >= fd

                                        Nothing ->
                                            True

                            toOk =
                                if String.isEmpty model.dateToFilter then
                                    True

                                else
                                    case dateToDays model.dateToFilter of
                                        Just td ->
                                            cd <= td

                                        Nothing ->
                                            True
                        in
                        fromOk && toOk
    in
    ownerOk && closeDateOk
