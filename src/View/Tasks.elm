module View.Tasks exposing (deleteTaskConfirmModal, taskFormModal, tasksView)

{-| Tasks page, task form and delete confirmation.
-}

import Html exposing (..)
import Html.Attributes as Attr exposing (class, disabled, for, id, placeholder, type_, value)
import Html.Events exposing (onClick, onInput, onSubmit)
import Json.Decode as D
import Types exposing (..)
import View.DealForm exposing (contactsForSelect)
import View.Icons exposing (iconEdit, iconTrash)


tasksSkeleton : Html Msg
tasksSkeleton =
    div []
        [ div [ class "page-toolbar" ]
            [ div [ class "page-toolbar__search" ]
                [ input [ type_ "text", placeholder "Search tasks…", disabled True ] [] ]
            ]
        , div [ class "table-wrap" ]
            (List.repeat 5
                (div [ class "skeleton-row" ]
                    [ div [ class "skeleton-line skeleton-line--long" ] []
                    , div [ class "skeleton-line skeleton-line--short" ] []
                    ]
                )
            )
        ]


taskStatusLabel : String -> String
taskStatusLabel s =
    case s of
        "todo" ->
            "To do"

        "in_progress" ->
            "In progress"

        "done" ->
            "Done"

        _ ->
            s


taskStatusClass : String -> String
taskStatusClass s =
    case s of
        "todo" ->
            "badge badge--muted"

        "in_progress" ->
            "badge badge--info"

        "done" ->
            "badge badge--success"

        _ ->
            "badge"


tasksView : Model -> Html Msg
tasksView model =
    case model.tasks of
        NotAsked ->
            tasksSkeleton

        Loading ->
            tasksSkeleton

        Failure msg ->
            div [ class "content__empty-block" ] [ text ("Could not load tasks: " ++ msg), button [ class "ecc-btn ecc-btn--ghost ecc-btn--inline", onClick (NavigatedTo model.route) ] [ text "Retry" ] ]

        Success data ->
            div []
                [ div [ class "page-toolbar" ]
                    [ div [ class "page-toolbar__search" ]
                        [ input
                            [ type_ "search"
                            , Attr.attribute "aria-label" "Search tasks"
                            , placeholder "Search tasks…"
                            , value data.query
                            , onInput UpdatedTasksQuery
                            ]
                            []
                        ]
                    , div [ class "stage-pills" ]
                        (List.map
                            (\s ->
                                let
                                    label =
                                        if s == "" then
                                            "All"

                                        else
                                            taskStatusLabel s

                                    cls =
                                        if data.statusFilter == s then
                                            "stage-pill stage-pill--active"

                                        else
                                            "stage-pill"
                                in
                                button [ type_ "button", class cls, onClick (UpdatedTasksStatusFilter s) ] [ text label ]
                            )
                            [ "", "todo", "in_progress", "done" ]
                        )
                    , button [ class "ecc-btn ecc-btn--inline", type_ "button", onClick OpenedAddTask ] [ text "Add task" ]
                    ]
                , if List.isEmpty data.items then
                    div [ class "content__empty-block" ] [ text "No tasks match. Add one to track follow-ups." ]

                  else
                    div [ class "table-wrap" ]
                        [ table [ class "data-table" ]
                            [ thead []
                                [ tr []
                                    [ th [] [ text "Task" ]
                                    , th [] [ text "Status" ]
                                    , th [] [ text "Due" ]
                                    , th [] [ text "Contact" ]
                                    , th [] [ text "" ]
                                    ]
                                ]
                            , tbody [] (List.map (taskRow model) data.items)
                            ]
                        , div [ class "workspace-toolbar" ]
                            [ button [ type_ "button", disabled (model.taskOffset == 0), onClick (ChangedTasksPage (model.taskOffset - 25)) ] [ text "Previous" ]
                            , span [] [ text ("Page " ++ String.fromInt (model.taskOffset // 25 + 1)) ]
                            , button [ type_ "button", disabled (model.taskOffset + 25 >= data.total), onClick (ChangedTasksPage (model.taskOffset + 25)) ] [ text "Next" ]
                            ]
                        , p [ class "page-toolbar__summary" ]
                            [ text
                                (String.fromInt data.total
                                    ++ " task"
                                    ++ (if data.total == 1 then
                                            ""

                                        else
                                            "s"
                                       )
                                )
                            ]
                        ]
                ]


taskRow : Model -> Task -> Html Msg
taskRow model task =
    tr []
        [ td []
            [ div [ class "cell-primary" ]
                [ strong [] [ text task.title ]
                , if String.isEmpty task.description then
                    text ""

                  else
                    span [ class "cell-secondary" ] [ text task.description ]
                ]
            ]
        , td []
            [ button
                [ type_ "button"
                , class (taskStatusClass task.status)
                , onClick
                    (ToggledTaskStatus task
                        (case task.status of
                            "todo" ->
                                "in_progress"

                            "in_progress" ->
                                "done"

                            _ ->
                                "todo"
                        )
                    )
                , Attr.title "Click to cycle status"
                ]
                [ text (taskStatusLabel task.status) ]
            ]
        , td []
            [ text
                (if String.isEmpty task.dueDate then
                    "—"

                 else
                    task.dueDate
                )
            ]
        , td [] [ contactLinkForTask model task ]
        , td [ class "cell-actions" ]
            [ button [ type_ "button", class "row-action", onClick (OpenedEditTask task), Attr.attribute "aria-label" "Edit" ] [ iconEdit ]
            , button [ type_ "button", class "row-action row-action--danger", onClick (RequestedDeleteTask task), Attr.attribute "aria-label" "Delete" ] [ iconTrash ]
            ]
        ]


contactLinkForTask : Model -> Task -> Html Msg
contactLinkForTask model task =
    if String.isEmpty task.contactId then
        text "—"

    else
        let
            found =
                case model.contacts of
                    Success d ->
                        List.filter (\c -> c.id == task.contactId) d.items |> List.head

                    _ ->
                        Nothing
        in
        case found of
            Just c ->
                button
                    [ type_ "button"
                    , class "ecc-link ecc-link--strong"
                    , onClick (OpenedContactDetail c)
                    ]
                    [ text
                        (if String.isEmpty task.contactName then
                            c.name

                         else
                            task.contactName
                        )
                    ]

            Nothing ->
                text
                    (if String.isEmpty task.contactName then
                        "—"

                     else
                        task.contactName
                    )


taskFormFieldError : String -> TaskForm -> Maybe String
taskFormFieldError field tf =
    tf.errors |> List.filter (\( f, _ ) -> f == field) |> List.head |> Maybe.map Tuple.second


taskFormModal : Model -> TaskForm -> Html Msg
taskFormModal model tf =
    let
        isEdit =
            model.editingTaskId /= Nothing

        titleText =
            if isEdit then
                "Edit task"

            else
                "Add task"

        allContacts =
            contactsForSelect model
    in
    div [ class "modal-backdrop", onClick RequestedCloseTaskForm ]
        [ div
            [ class "modal modal--wide"
            , Attr.attribute "role" "dialog"
            , Attr.attribute "data-draft-record-id" (Maybe.withDefault "new" model.editingTaskId)
            , Html.Events.stopPropagationOn "click" (D.succeed ( DismissedToast, True ))
            ]
            [ header [ class "modal__header" ]
                [ h2 [ class "modal__title" ] [ text titleText ]
                , button [ class "modal__close", type_ "button", onClick RequestedCloseTaskForm ] [ text "×" ]
                ]
            , if tf.confirmDiscard then
                div [ class "modal__confirm" ]
                    [ p [ class "modal__confirm-text" ] [ text "Discard changes?" ]
                    , div [ class "modal__actions" ]
                        [ button [ type_ "button", class "ecc-btn ecc-btn--ghost ecc-btn--inline", onClick CancelledCloseTaskForm ] [ text "Keep editing" ]
                        , button [ type_ "button", class "ecc-btn ecc-btn--danger ecc-btn--inline", onClick ConfirmedCloseTaskForm ] [ text "Discard" ]
                        ]
                    ]

              else
                form [ onSubmit SubmittedTaskForm, Attr.novalidate True ]
                    [ Html.node "crm-draft-recovery" [] []
                    , case taskFormFieldError "form" tf of
                        Just msg ->
                            div [ class "ecc-alert ecc-alert--error" ] [ text msg ]

                        Nothing ->
                            text ""
                    , div [ class "form-grid" ]
                        [ div
                            [ class
                                (if taskFormFieldError "title" tf /= Nothing then
                                    "ecc-field ecc-field--error"

                                 else
                                    "ecc-field"
                                )
                            ]
                            [ input
                                [ id "tf-title"
                                , type_ "text"
                                , placeholder " "
                                , value tf.title
                                , onInput (UpdatedTaskFormField "title")
                                , disabled tf.submitting
                                , Attr.autofocus True
                                ]
                                []
                            , label [ for "tf-title" ] [ text "Title" ]
                            ]
                        , div [ class "ecc-field" ]
                            [ input
                                [ id "tf-due"
                                , type_ "date"
                                , placeholder " "
                                , value tf.dueDate
                                , onInput (UpdatedTaskFormField "dueDate")
                                , disabled tf.submitting
                                ]
                                []
                            , label [ for "tf-due" ] [ text "Due date" ]
                            ]
                        ]
                    , div [ class "ecc-field" ]
                        [ span [ class "ecc-field__label" ] [ text "Contact" ]
                        , select
                            [ id "tf-contact"
                            , onInput (UpdatedTaskFormField "contactId")
                            , disabled tf.submitting
                            ]
                            (option [ value "", Attr.selected (tf.contactId == "") ] [ text "— None —" ]
                                :: List.map
                                    (\c -> option [ value c.id, Attr.selected (tf.contactId == c.id) ] [ text c.name ])
                                    allContacts
                            )
                        ]
                    , div [ class "ecc-field" ]
                        [ span [ class "ecc-field__label" ] [ text "Status" ]
                        , div [ class "stage-pills" ]
                            (List.map
                                (\s ->
                                    button
                                        [ type_ "button"
                                        , class
                                            (if tf.status == s then
                                                "stage-pill stage-pill--active"

                                             else
                                                "stage-pill"
                                            )
                                        , onClick (UpdatedTaskFormField "status" s)
                                        , disabled tf.submitting
                                        ]
                                        [ text (taskStatusLabel s) ]
                                )
                                taskStatuses
                            )
                        ]
                    , div [ class "ecc-field ecc-field--notes" ]
                        [ span [ class "ecc-field__label" ] [ text "Description" ]
                        , textarea
                            [ value tf.description
                            , onInput (UpdatedTaskFormField "description")
                            , disabled tf.submitting
                            , Attr.rows 3
                            , placeholder "What needs to be done?"
                            ]
                            []
                        ]
                    , div [ class "modal__actions" ]
                        [ button [ type_ "button", class "ecc-btn ecc-btn--ghost ecc-btn--inline", onClick RequestedCloseTaskForm, disabled tf.submitting ] [ text "Cancel" ]
                        , button
                            [ type_ "submit"
                            , class "ecc-btn ecc-btn--inline"
                            , disabled (tf.submitting || not tf.dirty)
                            ]
                            [ text
                                (if tf.submitting then
                                    "Saving…"

                                 else
                                    "Save task"
                                )
                            ]
                        ]
                    ]
            ]
        ]


deleteTaskConfirmModal : Task -> Html Msg
deleteTaskConfirmModal task =
    div [ class "modal-backdrop", onClick CancelledDeleteTask ]
        [ div
            [ class "modal modal--narrow"
            , Attr.attribute "role" "alertdialog"
            , Html.Events.stopPropagationOn "click" (D.succeed ( DismissedToast, True ))
            ]
            [ header [ class "modal__header" ]
                [ h2 [ class "modal__title" ] [ text "Delete task" ]
                ]
            , p [ class "modal__confirm-text" ]
                [ text "Delete "
                , strong [] [ text task.title ]
                , text "?"
                ]
            , div [ class "modal__actions" ]
                [ button [ type_ "button", class "ecc-btn ecc-btn--ghost ecc-btn--inline", onClick CancelledDeleteTask ] [ text "Cancel" ]
                , button [ type_ "button", class "ecc-btn ecc-btn--danger ecc-btn--inline", onClick ConfirmedDeleteTask ] [ text "Delete" ]
                ]
            ]
        ]
