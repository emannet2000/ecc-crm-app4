module Update.Drafts exposing (afterUpdate, update, hasOpen)

import Api
import Json.Decode as D
import Json.Encode as E
import Ports
import Router
import Types exposing (..)


type alias OpenDraft =
    { key : String, entity : String, recordId : String, dirty : Bool, submitting : Bool, fields : E.Value }


descriptor : Model -> String -> String -> Bool -> Bool -> E.Value -> OpenDraft
descriptor model entity recordId dirty submitting fields =
    { key = E.encode 0 (E.list E.string [ Maybe.withDefault "" (Maybe.map .id model.user), entity, recordId, if recordId == "new" then Router.routeToPath model.route else "" ])
    , entity = entity, recordId = recordId, dirty = dirty, submitting = submitting, fields = fields }


hasOpen : Model -> Bool
hasOpen model = active model /= Nothing


active : Model -> Maybe OpenDraft
active model =
    [ Nothing
        , Maybe.map (\form -> descriptor model "contacts" (Maybe.withDefault "new" model.editingId) form.dirty form.submitting (encodeContact form)) model.contactForm
        , Maybe.map (\form -> descriptor model "deals" (Maybe.withDefault "new" model.editingDealId) form.dirty form.submitting (encodeDeal form)) model.dealForm
        , Maybe.map (\form -> descriptor model "tasks" (Maybe.withDefault "new" model.editingTaskId) form.dirty form.submitting (encodeTask form)) model.taskForm
        , Maybe.map (\form -> descriptor model "schools" (Maybe.withDefault "new" model.editingSchoolId) form.dirty form.submitting (encodeSchool form)) model.schoolForm
        , Maybe.map (\form -> descriptor model "students" (Maybe.withDefault "new" model.editingStudentId) form.dirty form.submitting (encodeStudent form)) model.studentForm
        , Maybe.map (\form -> descriptor model "agents" (Maybe.withDefault "new" model.editingAgentId) form.dirty form.submitting (encodeAgent form)) model.agentForm
        , Maybe.map (\form -> descriptor model "leads" (Maybe.withDefault "new" model.editingLeadId) form.dirty form.submitting (encodeLead form)) model.leadForm
        , Maybe.map (\form -> descriptor model "cases" (Maybe.withDefault "new" model.editingCaseId) form.dirty form.submitting (encodeCase form)) model.caseForm
        , Maybe.map (\form -> descriptor model "documents" (Maybe.withDefault "new" model.editingDocumentId) form.dirty form.submitting (encodeDocument form)) model.documentForm
        , Maybe.map (\form -> descriptor model "invoices" (Maybe.withDefault "new" model.editingInvoiceId) form.dirty form.submitting (encodeInvoice form)) model.invoiceForm
        , Maybe.map (\form -> descriptor model "partners" (Maybe.withDefault "new" model.editingPartnerId) form.dirty form.submitting (encodePartner form)) model.partnerForm
        , Maybe.map (\form -> descriptor model "activities" ("new") form.dirty form.submitting (encodeActivity form)) model.activityForm
        , Maybe.map (\form -> descriptor model "payments" ("new") True form.submitting (encodePayment form)) model.paymentForm
    ] |> List.filterMap identity |> List.head


store : String -> OpenDraft -> Cmd Msg
store action draft =
    Ports.draftStorage { action = action, key = draft.key, fields = draft.fields }


afterUpdate : Msg -> Model -> Model -> ( Model, Cmd Msg )
afterUpdate msg before after =
    let
        saved =
            case msg of
                GotSavedContact (Ok record) -> Ports.recordSaved record.id
                GotSavedDeal (Ok record) -> Ports.recordSaved record.id
                GotSavedTask (Ok record) -> Ports.recordSaved record.id
                GotSavedSchool (Ok record) -> Ports.recordSaved record.id
                GotSavedStudent (Ok record) -> Ports.recordSaved record.id
                GotSavedAgent (Ok record) -> Ports.recordSaved record.id
                GotSavedLead (Ok record) -> Ports.recordSaved record.id
                GotSavedCase (Ok record) -> Ports.recordSaved record.id
                GotSavedDocument (Ok record) -> Ports.recordSaved record.id
                GotSavedInvoice (Ok record) -> Ports.recordSaved record.id
                GotSavedPartner (Ok record) -> Ports.recordSaved record.id
                GotSavedActivity (Ok record) -> Ports.recordSaved record.id
                GotSavedPayment (Ok record) -> Ports.recordSaved record.id
                GotMovedDeal (Ok record) -> Ports.recordSaved record.id
                GotToggledTask (Ok record) -> Ports.recordSaved record.id
                GotUpdatedDocument (Ok record) -> Ports.recordSaved record.id
                GotRefunded (Ok record) -> Ports.recordSaved record.id
                GotDeletedContact (Ok _) -> Ports.recordSaved ""
                GotDeletedDeal (Ok _) -> Ports.recordSaved ""
                GotDeletedTask (Ok _) -> Ports.recordSaved ""
                GotDeletedSchool (Ok _) -> Ports.recordSaved ""
                GotDeletedStudent (Ok _) -> Ports.recordSaved ""
                GotDeletedAgent (Ok _) -> Ports.recordSaved ""
                GotDeletedLead (Ok _) -> Ports.recordSaved ""
                GotDeletedCase (Ok _) -> Ports.recordSaved ""
                GotDeletedDocument (Ok _) -> Ports.recordSaved ""
                GotDeletedInvoice (Ok _) -> Ports.recordSaved ""
                GotDeletedPartner (Ok _) -> Ports.recordSaved ""
                GotDeletedActivity (Ok _) -> Ports.recordSaved ""
                GotDeletedPayment (Ok _) -> Ports.recordSaved ""
                GotBulkMoved _ (Ok record) -> Ports.recordSaved record.id
                GotBulkDeleted _ (Ok _) -> Ports.recordSaved ""
                _ -> Cmd.none
        ( next, storage ) = afterDraftUpdate msg before after
    in
    ( next, Cmd.batch [ saved, storage ] )


afterDraftUpdate : Msg -> Model -> Model -> ( Model, Cmd Msg )
afterDraftUpdate msg before after =
    case ( active before, active after ) of
        ( _, Just draft ) ->
            if failedSave msg then
                ( { after | draftRecovery = Just { key = draft.key, entity = draft.entity, recordId = draft.recordId, fields = draft.fields, current = draft.fields, restored = True, reviewing = False, reviewed = False, useLatest = False, error = Nothing } }, store "write" draft )

            else if Maybe.map .key (active before) /= Just draft.key then
                ( { after | draftRecovery = Nothing }, store "read" draft )

            else if draft.dirty && not draft.submitting && msg /= DiscardDraft && Maybe.map (E.encode 0 << .fields) (active before) /= Just (E.encode 0 draft.fields) then
                ( after, store "write" draft )

            else
                ( after, Cmd.none )

        ( Just draft, Nothing ) ->
            let
                discard =
                    case msg of
                        ConfirmedCloseContactForm -> True
                        ConfirmedCloseDealForm -> True
                        ConfirmedCloseTaskForm -> True
                        ConfirmedCloseSchoolForm -> True
                        ConfirmedCloseStudentForm -> True
                        ConfirmedCloseAgentForm -> True
                        ConfirmedCloseLeadForm -> True
                        ConfirmedCloseCaseForm -> True
                        ConfirmedCloseDocumentForm -> True
                        ConfirmedCloseInvoiceForm -> True
                        ConfirmedClosePartnerForm -> True
                        ConfirmedCloseActivityForm -> True
                        GotSavedContact (Ok _) -> True
                        GotSavedDeal (Ok _) -> True
                        GotSavedTask (Ok _) -> True
                        GotSavedSchool (Ok _) -> True
                        GotSavedStudent (Ok _) -> True
                        GotSavedAgent (Ok _) -> True
                        GotSavedLead (Ok _) -> True
                        GotSavedCase (Ok _) -> True
                        GotSavedDocument (Ok _) -> True
                        GotSavedInvoice (Ok _) -> True
                        GotSavedPartner (Ok _) -> True
                        GotSavedActivity (Ok _) -> True
                        GotSavedPayment (Ok _) -> True
                        _ -> False
            in
            ( { after | draftRecovery = Nothing }, if discard then store "remove" draft else Cmd.none )

        _ ->
            ( after, Cmd.none )


update : Msg -> Model -> ( Model, Cmd Msg )
update msg model =
    case msg of
        GotStoredDraft value ->
            case active model of
                Just draft ->
                    if read "key" D.string "" value == draft.key then
                        case D.decodeValue (D.field "fields" D.value) value of
                            Ok fields ->
                                if fields == E.null then
                                    ( model, Cmd.none )
                                else
                                    ( { model | draftRecovery = Just { key = draft.key, entity = draft.entity, recordId = draft.recordId, fields = fields, current = draft.fields, restored = False, reviewing = False, reviewed = False, useLatest = False, error = Nothing } }, Cmd.none )
                            Err _ -> ( model, Cmd.none )
                    else
                        ( model, Cmd.none )
                Nothing -> ( model, Cmd.none )

        RestoreDraft ->
            case model.draftRecovery of
                Just recovery ->
                    ( restoreForm recovery.entity recovery.fields { model | draftRecovery = Just { recovery | restored = True } }, Cmd.none )
                Nothing -> ( model, Cmd.none )

        DiscardDraft ->
            case active model of
                Just draft -> ( { model | draftRecovery = Nothing }, store "remove" draft )
                Nothing -> ( model, Cmd.none )

        ReviewLatestDraft ->
            case ( model.draftRecovery, model.token ) of
                ( Just recovery, Just token ) ->
                    ( { model | draftRecovery = Just { recovery | reviewing = True, reviewed = False, useLatest = False, error = Nothing } }
                    , Api.workspaceGet token ("/api/records/" ++ recovery.entity ++ "/" ++ recovery.recordId) (D.field "record" D.value) (GotLatestDraft recovery.key)
                    )
                _ -> ( model, Cmd.none )

        GotLatestDraft key result ->
            case model.draftRecovery of
                Just recovery ->
                    if recovery.key /= key then ( model, Cmd.none )
                    else
                        case result |> Result.andThen (latestForm recovery.entity) of
                            Ok fields ->
                                ( { model | draftRecovery = Just { recovery | current = fields, reviewing = False, reviewed = True, error = Nothing } }, Cmd.none )
                            Err error ->
                                ( { model | draftRecovery = Just { recovery | reviewing = False, error = Just error } }, Cmd.none )
                Nothing -> ( model, Cmd.none )

        UseLatestDraftRevision useLatest ->
            case ( model.draftRecovery, active model ) of
                ( Just recovery, Just draft ) ->
                    if not recovery.reviewed || not recovery.restored then ( model, Cmd.none )
                    else
                        let
                            revision = read "revision" D.string "" (if useLatest then recovery.current else recovery.fields)
                            fields = patchRevision revision draft.fields
                        in
                        ( restoreForm recovery.entity fields { model | draftRecovery = Just { recovery | useLatest = useLatest } }, Cmd.none )
                _ -> ( model, Cmd.none )

        _ -> ( model, Cmd.none )


read : String -> D.Decoder a -> a -> E.Value -> a
read key decoder fallback value =
    D.decodeValue (D.field key decoder) value |> Result.withDefault fallback


patchRevision : String -> E.Value -> E.Value
patchRevision revision fields =
    E.object ((D.decodeValue (D.keyValuePairs D.value) fields |> Result.withDefault [] |> List.filter (\( key, _ ) -> key /= "revision")) ++ [ ( "revision", E.string revision ) ])


restoreForm : String -> E.Value -> Model -> Model
restoreForm entity fields model =
    case entity of
        "contacts" -> { model | contactForm = Maybe.map (restoreContact fields) model.contactForm }
        "deals" -> { model | dealForm = Maybe.map (restoreDeal fields) model.dealForm }
        "tasks" -> { model | taskForm = Maybe.map (restoreTask fields) model.taskForm }
        "schools" -> { model | schoolForm = Maybe.map (restoreSchool fields) model.schoolForm }
        "students" -> { model | studentForm = Maybe.map (restoreStudent fields) model.studentForm }
        "agents" -> { model | agentForm = Maybe.map (restoreAgent fields) model.agentForm }
        "leads" -> { model | leadForm = Maybe.map (restoreLead fields) model.leadForm }
        "cases" -> { model | caseForm = Maybe.map (restoreCase fields) model.caseForm }
        "documents" -> { model | documentForm = Maybe.map (restoreDocument fields) model.documentForm }
        "invoices" -> { model | invoiceForm = Maybe.map (restoreInvoice fields) model.invoiceForm }
        "partners" -> { model | partnerForm = Maybe.map (restorePartner fields) model.partnerForm }
        "activities" -> { model | activityForm = Maybe.map (restoreActivity fields) model.activityForm }
        "payments" -> { model | paymentForm = Maybe.map (restorePayment fields) model.paymentForm }
        _ -> model


latestForm : String -> E.Value -> Result String E.Value
latestForm entity value =
    if not (read "_editable" D.bool False value) then
        Err "You no longer have permission to edit this record."
    else
        case entity of
            "contacts" -> D.decodeValue (D.map (contactToForm >> encodeContact) Api.contactDecoder) value |> Result.mapError D.errorToString
            "deals" -> D.decodeValue (D.map (dealToForm >> encodeDeal) Api.dealDecoder) value |> Result.mapError D.errorToString
            "tasks" -> D.decodeValue (D.map (taskToForm >> encodeTask) Api.taskDecoder) value |> Result.mapError D.errorToString
            "schools" -> D.decodeValue (D.map (schoolToForm >> encodeSchool) Api.schoolDecoder) value |> Result.mapError D.errorToString
            "students" -> D.decodeValue (D.map (studentToForm >> encodeStudent) Api.studentDecoder) value |> Result.mapError D.errorToString
            "agents" -> D.decodeValue (D.map (agentToForm >> encodeAgent) Api.agentDecoder) value |> Result.mapError D.errorToString
            "leads" -> D.decodeValue (D.map (leadToForm >> encodeLead) Api.leadDecoder) value |> Result.mapError D.errorToString
            "cases" -> D.decodeValue (D.map (caseToForm >> encodeCase) Api.caseDecoder) value |> Result.mapError D.errorToString
            "documents" -> D.decodeValue (D.map (documentToForm >> encodeDocument) Api.documentDecoder) value |> Result.mapError D.errorToString
            "invoices" -> D.decodeValue (D.map (invoiceToForm >> encodeInvoice) Api.invoiceDecoder) value |> Result.mapError D.errorToString
            "partners" -> D.decodeValue (D.map (partnerToForm >> encodePartner) Api.partnerDecoder) value |> Result.mapError D.errorToString
            _ -> Err "This draft is for a new record."


encodeContact : ContactForm -> E.Value
encodeContact form =
    E.object
        [ ( "revision", E.string form.revision )
        , ( "name", E.string form.name )
        , ( "email", E.string form.email )
        , ( "company", E.string form.company )
        , ( "title", E.string form.title )
        , ( "phone", E.string form.phone )
        , ( "location", E.string form.location )
        , ( "stage", E.string form.stage )
        , ( "tags", E.list E.string form.tags )
        , ( "tagInput", E.string form.tagInput )
        , ( "notes", E.string form.notes )
        ]


restoreContact : E.Value -> ContactForm -> ContactForm
restoreContact fields form =
    { form
        | revision = read "revision" D.string form.revision fields
        , name = read "name" D.string form.name fields
        , email = read "email" D.string form.email fields
        , company = read "company" D.string form.company fields
        , title = read "title" D.string form.title fields
        , phone = read "phone" D.string form.phone fields
        , location = read "location" D.string form.location fields
        , stage = read "stage" D.string form.stage fields
        , tags = read "tags" (D.list D.string) form.tags fields
        , tagInput = read "tagInput" D.string form.tagInput fields
        , notes = read "notes" D.string form.notes fields
        , dirty = True
        , submitting = False
        , errors = []
    }


encodeDeal : DealForm -> E.Value
encodeDeal form =
    E.object
        [ ( "revision", E.string form.revision )
        , ( "title", E.string form.title )
        , ( "contactId", E.string form.contactId )
        , ( "value", E.string form.value )
        , ( "currency", E.string form.currency )
        , ( "stage", E.string form.stage )
        , ( "closeDate", E.string form.closeDate )
        , ( "owner", E.string form.owner )
        , ( "notes", E.string form.notes )
        ]


restoreDeal : E.Value -> DealForm -> DealForm
restoreDeal fields form =
    { form
        | revision = read "revision" D.string form.revision fields
        , title = read "title" D.string form.title fields
        , contactId = read "contactId" D.string form.contactId fields
        , value = read "value" D.string form.value fields
        , currency = read "currency" D.string form.currency fields
        , stage = read "stage" D.string form.stage fields
        , closeDate = read "closeDate" D.string form.closeDate fields
        , owner = read "owner" D.string form.owner fields
        , notes = read "notes" D.string form.notes fields
        , dirty = True
        , submitting = False
        , errors = []
    }


encodeTask : TaskForm -> E.Value
encodeTask form =
    E.object
        [ ( "revision", E.string form.revision )
        , ( "title", E.string form.title )
        , ( "description", E.string form.description )
        , ( "status", E.string form.status )
        , ( "dueDate", E.string form.dueDate )
        , ( "contactId", E.string form.contactId )
        , ( "owner", E.string form.owner )
        ]


restoreTask : E.Value -> TaskForm -> TaskForm
restoreTask fields form =
    { form
        | revision = read "revision" D.string form.revision fields
        , title = read "title" D.string form.title fields
        , description = read "description" D.string form.description fields
        , status = read "status" D.string form.status fields
        , dueDate = read "dueDate" D.string form.dueDate fields
        , contactId = read "contactId" D.string form.contactId fields
        , owner = read "owner" D.string form.owner fields
        , dirty = True
        , submitting = False
        , errors = []
    }


encodeSchool : SchoolForm -> E.Value
encodeSchool form =
    E.object
        [ ( "revision", E.string form.revision )
        , ( "name", E.string form.name )
        , ( "countryCode", E.string form.countryCode )
        , ( "commissionRate", E.string form.commissionRate )
        , ( "contractStatus", E.string form.contractStatus )
        , ( "studentsEnrolled", E.string form.studentsEnrolled )
        , ( "contactPerson", E.string form.contactPerson )
        , ( "website", E.string form.website )
        , ( "notes", E.string form.notes )
        ]


restoreSchool : E.Value -> SchoolForm -> SchoolForm
restoreSchool fields form =
    { form
        | revision = read "revision" D.string form.revision fields
        , name = read "name" D.string form.name fields
        , countryCode = read "countryCode" D.string form.countryCode fields
        , commissionRate = read "commissionRate" D.string form.commissionRate fields
        , contractStatus = read "contractStatus" D.string form.contractStatus fields
        , studentsEnrolled = read "studentsEnrolled" D.string form.studentsEnrolled fields
        , contactPerson = read "contactPerson" D.string form.contactPerson fields
        , website = read "website" D.string form.website fields
        , notes = read "notes" D.string form.notes fields
        , dirty = True
        , submitting = False
        , errors = []
    }


encodeStudent : StudentForm -> E.Value
encodeStudent form =
    E.object
        [ ( "revision", E.string form.revision )
        , ( "name", E.string form.name )
        , ( "studentCode", E.string form.studentCode )
        , ( "countryCode", E.string form.countryCode )
        , ( "schoolId", E.string form.schoolId )
        , ( "agentId", E.string form.agentId )
        , ( "program", E.string form.program )
        , ( "acceptanceStatus", E.string form.acceptanceStatus )
        , ( "visaStatus", E.string form.visaStatus )
        , ( "invoiceStatus", E.string form.invoiceStatus )
        , ( "notes", E.string form.notes )
        ]


restoreStudent : E.Value -> StudentForm -> StudentForm
restoreStudent fields form =
    { form
        | revision = read "revision" D.string form.revision fields
        , name = read "name" D.string form.name fields
        , studentCode = read "studentCode" D.string form.studentCode fields
        , countryCode = read "countryCode" D.string form.countryCode fields
        , schoolId = read "schoolId" D.string form.schoolId fields
        , agentId = read "agentId" D.string form.agentId fields
        , program = read "program" D.string form.program fields
        , acceptanceStatus = read "acceptanceStatus" D.string form.acceptanceStatus fields
        , visaStatus = read "visaStatus" D.string form.visaStatus fields
        , invoiceStatus = read "invoiceStatus" D.string form.invoiceStatus fields
        , notes = read "notes" D.string form.notes fields
        , dirty = True
        , submitting = False
        , errors = []
    }


encodeAgent : AgentForm -> E.Value
encodeAgent form =
    E.object
        [ ( "revision", E.string form.revision )
        , ( "name", E.string form.name )
        , ( "agentCode", E.string form.agentCode )
        , ( "countryCode", E.string form.countryCode )
        , ( "contractStatus", E.string form.contractStatus )
        , ( "agentStatus", E.string form.agentStatus )
        , ( "studentsReferred", E.string form.studentsReferred )
        , ( "notes", E.string form.notes )
        ]


restoreAgent : E.Value -> AgentForm -> AgentForm
restoreAgent fields form =
    { form
        | revision = read "revision" D.string form.revision fields
        , name = read "name" D.string form.name fields
        , agentCode = read "agentCode" D.string form.agentCode fields
        , countryCode = read "countryCode" D.string form.countryCode fields
        , contractStatus = read "contractStatus" D.string form.contractStatus fields
        , agentStatus = read "agentStatus" D.string form.agentStatus fields
        , studentsReferred = read "studentsReferred" D.string form.studentsReferred fields
        , notes = read "notes" D.string form.notes fields
        , dirty = True
        , submitting = False
        , errors = []
    }


encodeLead : LeadForm -> E.Value
encodeLead form =
    E.object
        [ ( "revision", E.string form.revision )
        , ( "leadNumber", E.string form.leadNumber )
        , ( "name", E.string form.name )
        , ( "email", E.string form.email )
        , ( "phone", E.string form.phone )
        , ( "nationality", E.string form.nationality )
        , ( "currentCountry", E.string form.currentCountry )
        , ( "interestedCountry", E.string form.interestedCountry )
        , ( "interestedService", E.string form.interestedService )
        , ( "source", E.string form.source )
        , ( "referrer", E.string form.referrer )
        , ( "assignedTo", E.string form.assignedTo )
        , ( "status", E.string form.status )
        , ( "followUpDate", E.string form.followUpDate )
        , ( "nextAction", E.string form.nextAction )
        , ( "notes", E.string form.notes )
        ]


restoreLead : E.Value -> LeadForm -> LeadForm
restoreLead fields form =
    { form
        | revision = read "revision" D.string form.revision fields
        , leadNumber = read "leadNumber" D.string form.leadNumber fields
        , name = read "name" D.string form.name fields
        , email = read "email" D.string form.email fields
        , phone = read "phone" D.string form.phone fields
        , nationality = read "nationality" D.string form.nationality fields
        , currentCountry = read "currentCountry" D.string form.currentCountry fields
        , interestedCountry = read "interestedCountry" D.string form.interestedCountry fields
        , interestedService = read "interestedService" D.string form.interestedService fields
        , source = read "source" D.string form.source fields
        , referrer = read "referrer" D.string form.referrer fields
        , assignedTo = read "assignedTo" D.string form.assignedTo fields
        , status = read "status" D.string form.status fields
        , followUpDate = read "followUpDate" D.string form.followUpDate fields
        , nextAction = read "nextAction" D.string form.nextAction fields
        , notes = read "notes" D.string form.notes fields
        , dirty = True
        , submitting = False
        , errors = []
    }


encodeCase : CaseForm -> E.Value
encodeCase form =
    E.object
        [ ( "revision", E.string form.revision )
        , ( "caseNumber", E.string form.caseNumber )
        , ( "clientId", E.string form.clientId )
        , ( "studentId", E.string form.studentId )
        , ( "serviceCategory", E.string form.serviceCategory )
        , ( "destinationCountry", E.string form.destinationCountry )
        , ( "visaType", E.string form.visaType )
        , ( "schoolOrEmployer", E.string form.schoolOrEmployer )
        , ( "assignedOfficer", E.string form.assignedOfficer )
        , ( "externalAdviser", E.string form.externalAdviser )
        , ( "dateOpened", E.string form.dateOpened )
        , ( "targetSubmission", E.string form.targetSubmission )
        , ( "actualSubmission", E.string form.actualSubmission )
        , ( "governmentRef", E.string form.governmentRef )
        , ( "currentStage", E.string form.currentStage )
        , ( "priority", E.string form.priority )
        , ( "nextAction", E.string form.nextAction )
        , ( "nextDeadline", E.string form.nextDeadline )
        , ( "result", E.string form.result )
        , ( "closureDate", E.string form.closureDate )
        , ( "notes", E.string form.notes )
        ]


restoreCase : E.Value -> CaseForm -> CaseForm
restoreCase fields form =
    { form
        | revision = read "revision" D.string form.revision fields
        , caseNumber = read "caseNumber" D.string form.caseNumber fields
        , clientId = read "clientId" D.string form.clientId fields
        , studentId = read "studentId" D.string form.studentId fields
        , serviceCategory = read "serviceCategory" D.string form.serviceCategory fields
        , destinationCountry = read "destinationCountry" D.string form.destinationCountry fields
        , visaType = read "visaType" D.string form.visaType fields
        , schoolOrEmployer = read "schoolOrEmployer" D.string form.schoolOrEmployer fields
        , assignedOfficer = read "assignedOfficer" D.string form.assignedOfficer fields
        , externalAdviser = read "externalAdviser" D.string form.externalAdviser fields
        , dateOpened = read "dateOpened" D.string form.dateOpened fields
        , targetSubmission = read "targetSubmission" D.string form.targetSubmission fields
        , actualSubmission = read "actualSubmission" D.string form.actualSubmission fields
        , governmentRef = read "governmentRef" D.string form.governmentRef fields
        , currentStage = read "currentStage" D.string form.currentStage fields
        , priority = read "priority" D.string form.priority fields
        , nextAction = read "nextAction" D.string form.nextAction fields
        , nextDeadline = read "nextDeadline" D.string form.nextDeadline fields
        , result = read "result" D.string form.result fields
        , closureDate = read "closureDate" D.string form.closureDate fields
        , notes = read "notes" D.string form.notes fields
        , dirty = True
        , submitting = False
        , errors = []
    }


encodeDocument : DocumentForm -> E.Value
encodeDocument form =
    E.object
        [ ( "revision", E.string form.revision )
        , ( "docName", E.string form.docName )
        , ( "dateRequested", E.string form.dateRequested )
        , ( "dateReceived", E.string form.dateReceived )
        , ( "expiryDate", E.string form.expiryDate )
        , ( "verifiedBy", E.string form.verifiedBy )
        , ( "verificationDate", E.string form.verificationDate )
        , ( "status", E.string form.status )
        , ( "rejectionReason", E.string form.rejectionReason )
        , ( "notes", E.string form.notes )
        ]


restoreDocument : E.Value -> DocumentForm -> DocumentForm
restoreDocument fields form =
    { form
        | revision = read "revision" D.string form.revision fields
        , docName = read "docName" D.string form.docName fields
        , dateRequested = read "dateRequested" D.string form.dateRequested fields
        , dateReceived = read "dateReceived" D.string form.dateReceived fields
        , expiryDate = read "expiryDate" D.string form.expiryDate fields
        , verifiedBy = read "verifiedBy" D.string form.verifiedBy fields
        , verificationDate = read "verificationDate" D.string form.verificationDate fields
        , status = read "status" D.string form.status fields
        , rejectionReason = read "rejectionReason" D.string form.rejectionReason fields
        , notes = read "notes" D.string form.notes fields
        , dirty = True
        , submitting = False
        , errors = []
    }


encodeInvoice : InvoiceForm -> E.Value
encodeInvoice form =
    E.object
        [ ( "revision", E.string form.revision )
        , ( "invoiceNumber", E.string form.invoiceNumber )
        , ( "clientId", E.string form.clientId )
        , ( "caseId", E.string form.caseId )
        , ( "totalFee", E.string form.totalFee )
        , ( "governmentFee", E.string form.governmentFee )
        , ( "schoolPartnerFee", E.string form.schoolPartnerFee )
        , ( "amountReceived", E.string form.amountReceived )
        , ( "paymentMilestone", E.string form.paymentMilestone )
        , ( "paymentMethod", E.string form.paymentMethod )
        , ( "officialReceiptNumber", E.string form.officialReceiptNumber )
        , ( "refundStatus", E.string form.refundStatus )
        , ( "referralCommission", E.string form.referralCommission )
        , ( "partnerPayable", E.string form.partnerPayable )
        , ( "paymentApproval", E.string form.paymentApproval )
        , ( "notes", E.string form.notes )
        ]


restoreInvoice : E.Value -> InvoiceForm -> InvoiceForm
restoreInvoice fields form =
    { form
        | revision = read "revision" D.string form.revision fields
        , invoiceNumber = read "invoiceNumber" D.string form.invoiceNumber fields
        , clientId = read "clientId" D.string form.clientId fields
        , caseId = read "caseId" D.string form.caseId fields
        , totalFee = read "totalFee" D.string form.totalFee fields
        , governmentFee = read "governmentFee" D.string form.governmentFee fields
        , schoolPartnerFee = read "schoolPartnerFee" D.string form.schoolPartnerFee fields
        , amountReceived = read "amountReceived" D.string form.amountReceived fields
        , paymentMilestone = read "paymentMilestone" D.string form.paymentMilestone fields
        , paymentMethod = read "paymentMethod" D.string form.paymentMethod fields
        , officialReceiptNumber = read "officialReceiptNumber" D.string form.officialReceiptNumber fields
        , refundStatus = read "refundStatus" D.string form.refundStatus fields
        , referralCommission = read "referralCommission" D.string form.referralCommission fields
        , partnerPayable = read "partnerPayable" D.string form.partnerPayable fields
        , paymentApproval = read "paymentApproval" D.string form.paymentApproval fields
        , notes = read "notes" D.string form.notes fields
        , dirty = True
        , submitting = False
        , errors = []
    }


encodePartner : PartnerForm -> E.Value
encodePartner form =
    E.object
        [ ( "revision", E.string form.revision )
        , ( "type_", E.string form.type_ )
        , ( "legalCompanyName", E.string form.legalCompanyName )
        , ( "country", E.string form.country )
        , ( "licenseNumber", E.string form.licenseNumber )
        , ( "licenseExpiry", E.string form.licenseExpiry )
        , ( "verificationSource", E.string form.verificationSource )
        , ( "contactPerson", E.string form.contactPerson )
        , ( "contactEmail", E.string form.contactEmail )
        , ( "contactPhone", E.string form.contactPhone )
        , ( "agreementStart", E.string form.agreementStart )
        , ( "agreementExpiry", E.string form.agreementExpiry )
        , ( "servicesPermitted", E.string form.servicesPermitted )
        , ( "commissionStructure", E.string form.commissionStructure )
        , ( "paymentTerms", E.string form.paymentTerms )
        , ( "casesReferred", E.string form.casesReferred )
        , ( "casesConverted", E.string form.casesConverted )
        , ( "amountPayable", E.string form.amountPayable )
        , ( "complianceNotes", E.string form.complianceNotes )
        , ( "notes", E.string form.notes )
        ]


restorePartner : E.Value -> PartnerForm -> PartnerForm
restorePartner fields form =
    { form
        | revision = read "revision" D.string form.revision fields
        , type_ = read "type_" D.string form.type_ fields
        , legalCompanyName = read "legalCompanyName" D.string form.legalCompanyName fields
        , country = read "country" D.string form.country fields
        , licenseNumber = read "licenseNumber" D.string form.licenseNumber fields
        , licenseExpiry = read "licenseExpiry" D.string form.licenseExpiry fields
        , verificationSource = read "verificationSource" D.string form.verificationSource fields
        , contactPerson = read "contactPerson" D.string form.contactPerson fields
        , contactEmail = read "contactEmail" D.string form.contactEmail fields
        , contactPhone = read "contactPhone" D.string form.contactPhone fields
        , agreementStart = read "agreementStart" D.string form.agreementStart fields
        , agreementExpiry = read "agreementExpiry" D.string form.agreementExpiry fields
        , servicesPermitted = read "servicesPermitted" D.string form.servicesPermitted fields
        , commissionStructure = read "commissionStructure" D.string form.commissionStructure fields
        , paymentTerms = read "paymentTerms" D.string form.paymentTerms fields
        , casesReferred = read "casesReferred" D.string form.casesReferred fields
        , casesConverted = read "casesConverted" D.string form.casesConverted fields
        , amountPayable = read "amountPayable" D.string form.amountPayable fields
        , complianceNotes = read "complianceNotes" D.string form.complianceNotes fields
        , notes = read "notes" D.string form.notes fields
        , dirty = True
        , submitting = False
        , errors = []
    }


encodeActivity : ActivityForm -> E.Value
encodeActivity form =
    E.object
        [ ( "kind", E.string form.kind )
        , ( "title", E.string form.title )
        , ( "body", E.string form.body )
        , ( "occurredAt", E.string form.occurredAt )
        ]


restoreActivity : E.Value -> ActivityForm -> ActivityForm
restoreActivity fields form =
    { form
        | kind = read "kind" D.string form.kind fields
        , title = read "title" D.string form.title fields
        , body = read "body" D.string form.body fields
        , occurredAt = read "occurredAt" D.string form.occurredAt fields
        , dirty = True
        , submitting = False
        , errors = []
    }


encodePayment : PaymentForm -> E.Value
encodePayment form =
    E.object
        [ ( "amount", E.string form.amount )
        , ( "paidOn", E.string form.paidOn )
        , ( "method", E.string form.method )
        , ( "reference", E.string form.reference )
        , ( "notes", E.string form.notes )
        ]


restorePayment : E.Value -> PaymentForm -> PaymentForm
restorePayment fields form =
    { form
        | amount = read "amount" D.string form.amount fields
        , paidOn = read "paidOn" D.string form.paidOn fields
        , method = read "method" D.string form.method fields
        , reference = read "reference" D.string form.reference fields
        , notes = read "notes" D.string form.notes fields
        , submitting = False
        , errors = []
    }


failedSave : Msg -> Bool
failedSave msg =
    case msg of
        GotSavedContact (Err _) -> True
        GotSavedDeal (Err _) -> True
        GotSavedTask (Err _) -> True
        GotSavedSchool (Err _) -> True
        GotSavedStudent (Err _) -> True
        GotSavedAgent (Err _) -> True
        GotSavedLead (Err _) -> True
        GotSavedCase (Err _) -> True
        GotSavedDocument (Err _) -> True
        GotSavedInvoice (Err _) -> True
        GotSavedPartner (Err _) -> True
        GotSavedActivity (Err _) -> True
        GotSavedPayment (Err _) -> True
        _ -> False
